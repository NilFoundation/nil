package journald_forwarder

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/rs/zerolog"
	logs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	v1 "go.opentelemetry.io/proto/otlp/common/v1"
	v12 "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/grpc"
)

type Record struct {
	Type  string
	Value any
}

type Event map[string]Record

func (e Event) GetColumnNames() []string {
	res := make([]string, 0, len(e))
	for key := range e {
		res = append(res, key)
	}
	return res
}

type LogServer struct {
	logs.UnimplementedLogsServiceServer
	click  *Clickhouse
	logger logging.Logger
}

func NewLogServer(click *Clickhouse, logger logging.Logger) *LogServer {
	return &LogServer{click: click, logger: logger}
}

var fieldStoreClickhouseTyped = logging.FieldStoreToClickhouse + logging.GetAbbreviation("bool")

func createUniqueColumnsEvent(events []Event) Event {
	uniqueColumns := Event{}
	for _, event := range events {
		for key, value := range event {
			if _, exists := uniqueColumns[key]; !exists {
				uniqueColumns[key] = value
			}
		}
	}
	return uniqueColumns
}

func extractLogColumns(data map[string]any) Event {
	res := Event{}
	for key, value := range data {
		baseColumn := key[:len(key)-logging.LogAbbreviationSize]
		columnType := key[len(key)-logging.LogAbbreviationSize:]
		res[baseColumn] = Record{
			Type:  columnType,
			Value: value,
		}
	}
	return res
}

func preProcessData(logger logging.Logger, data map[string]any) (Event, error) {
	delete(data, fieldStoreClickhouseTyped)
	for _, field := range []string{
		zerolog.CallerFieldName,
		zerolog.MessageFieldName,
		zerolog.LevelFieldName,
		zerolog.ErrorFieldName,
		zerolog.ErrorStackFieldName,
		logging.FieldHostName,
		logging.FieldSystemdUnit,
	} {
		if value, ok := data[field]; ok {
			data[field+logging.GetAbbreviation("string")] = value
			delete(data, field)
		}
	}

	if value, ok := data[zerolog.TimestampFieldName]; ok {
		strValue, ok := value.(string)
		if !ok {
			logger.Error().Msgf("timestamp is not a string in log %+v", data)
			return nil, errors.New("timestamp is not a string")
		}
		parsedTime, err := time.Parse(time.RFC3339, strValue)
		if err != nil {
			logger.Error().Err(err).Msg("Error parsing timestamp")
			return nil, err
		}
		data[zerolog.TimestampFieldName+logging.GetAbbreviation("datetime64")] = parsedTime
		delete(data, zerolog.TimestampFieldName)
	}

	return extractLogColumns(data), nil
}

func storeData(ctx context.Context, click *Clickhouse, data []Event) error {
	if len(data) == 0 {
		return nil
	}
	uniqueEvent := createUniqueColumnsEvent(data)
	names := uniqueEvent.GetColumnNames()
	values := make([][]any, len(data))
	for i, d := range data {
		values[i] = make([]any, 0, len(d))
		for _, name := range names {
			if value, ok := d[name]; ok {
				values[i] = append(values[i], value.Value)
			} else {
				values[i] = append(values[i], nil)
			}
		}
	}

	return click.InsertData(ctx, DefaultDatabase, DefaultTable, names, values)
}

func (s *LogServer) processResourceLog(resourceLog *v12.ResourceLogs) ([]Event, error) {
	var res []Event
	for _, scopeLog := range resourceLog.ScopeLogs {
		log, err := s.processScopeLog(scopeLog)
		if err != nil {
			return nil, err
		}
		res = append(res, log...)
	}
	return res, nil
}

func (s *LogServer) processScopeLog(scopeLog *v12.ScopeLogs) ([]Event, error) {
	var res []Event
	for _, logRecord := range scopeLog.LogRecords {
		log, err := s.processLogRecord(logRecord)
		if err != nil {
			return nil, err
		}
		if log != nil {
			res = append(res, log)
		}
	}
	return res, nil
}

func (s *LogServer) processLogRecord(logRecord *v12.LogRecord) (Event, error) {
	var jsonData *v1.AnyValue
	var hostname string
	var unit string
	for _, kv := range logRecord.Body.GetKvlistValue().GetValues() {
		switch kv.GetKey() {
		case "JSON":
			jsonData = kv.GetValue()
		case logging.FieldHostName:
			hostname = kv.GetValue().GetStringValue()
		case logging.FieldSystemdUnit:
			unit = kv.GetValue().GetStringValue()
		}
	}
	if jsonData == nil || jsonData.GetStringValue() == "" {
		return nil, nil
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonData.GetStringValue()), &data); err != nil {
		s.logger.Error().Err(err).Msgf("Error parsing JSON in log %+v", data)
		return nil, err
	}

	if data[fieldStoreClickhouseTyped] == false {
		return nil, nil
	}

	data[logging.FieldHostName] = hostname
	data[logging.FieldSystemdUnit] = unit

	res, err := preProcessData(s.logger, data)
	if err != nil {
		return nil, err
	}
	return res, err
}

func (s *LogServer) Export(
	ctx context.Context, req *logs.ExportLogsServiceRequest,
) (*logs.ExportLogsServiceResponse, error) {
	var eventsToStore []Event
	for _, resourceLog := range req.ResourceLogs {
		log, err := s.processResourceLog(resourceLog)
		if err != nil {
			return nil, err
		}
		eventsToStore = append(eventsToStore, log...)
	}

	var insertErr error
	if insertErr = storeData(ctx, s.click, eventsToStore); insertErr == nil {
		return &logs.ExportLogsServiceResponse{}, nil
	}

	existColumns, err := s.click.GetTabelColumnNames(ctx, DefaultDatabase, DefaultTable)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error getting table columns")
		return nil, err
	}

	uniqueEvent := createUniqueColumnsEvent(eventsToStore)
	for key := range existColumns {
		delete(uniqueEvent, key)
	}

	if len(uniqueEvent) == 0 {
		s.logger.Error().Err(insertErr).Msgf("Error inserting data: %+v", eventsToStore)
		return nil, err
	}

	diffNames := make([]string, 0, len(uniqueEvent))
	diffTypes := make([]string, 0, len(uniqueEvent))
	for key, value := range uniqueEvent {
		chType, err := logging.GetClickhouseByAbbreviation(value.Type)
		if err != nil {
			s.logger.Error().Err(err).Msgf("Clickhouse type error: log %+v", eventsToStore)
			return nil, err
		}
		diffNames = append(diffNames, key)
		diffTypes = append(diffTypes, chType)
	}

	if err := s.click.InsertColumnsInTable(ctx, DefaultDatabase, DefaultTable, diffNames, diffTypes); err != nil {
		s.logger.Error().Err(err).Msgf("Error inserting columns names: %+v, types: %+v", diffNames, diffTypes)
		return nil, err
	}

	if err := storeData(ctx, s.click, eventsToStore); err != nil {
		s.logger.Error().Err(insertErr).Msgf("Error inserting data: %+v", eventsToStore)
		return nil, err
	}

	return &logs.ExportLogsServiceResponse{}, nil
}

func Run(ctx context.Context, cfg ClickhouseConfig, logger logging.Logger) error {
	click := NewClickhouse(cfg)
	if err := click.Connect(); err != nil {
		logger.Error().Err(err).Msg("Failed to connection to ClickHouse")
		return err
	}
	defer click.Close()

	if err := click.InitializeDatabaseSchema(ctx, logger); err != nil {
		return err
	}

	server := grpc.NewServer()
	logs.RegisterLogsServiceServer(server, NewLogServer(click, logger))

	var listener net.Listener
	var err error
	unixPrefix := "unix://"
	if strings.HasPrefix(cfg.ListenAddr, unixPrefix) {
		listener, err = net.Listen("unix", cfg.ListenAddr[len(unixPrefix):])
	} else {
		listener, err = net.Listen("tcp", cfg.ListenAddr)
	}

	if err != nil {
		logger.Error().Err(err).Msgf("Failed to listen on port %s", cfg.ListenAddr)
		return err
	}
	logger.Info().Str("listen_addr", cfg.ListenAddr).Msg("Log receiver listening")

	serverErrChan := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil {
			serverErrChan <- err
		}
		close(serverErrChan)
	}()

	select {
	case <-ctx.Done():
		logger.Info().Msg("Shutting down gRPC server due to context cancellation")
		server.GracefulStop()
		return nil
	case err := <-serverErrChan:
		logger.Error().Err(err).Msg("gRPC server failed")
		return err
	}
}
