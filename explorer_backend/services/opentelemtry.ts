import { MeterProvider } from "@opentelemetry/sdk-metrics";
import { NodeTracerProvider, TraceIdRatioBasedSampler } from "@opentelemetry/sdk-trace-node";
import { registerInstrumentations } from "@opentelemetry/instrumentation";
import { PeriodicExportingMetricReader, ConsoleMetricExporter } from "@opentelemetry/sdk-metrics";

import opentelemetry from "@opentelemetry/api";
import { HttpInstrumentation } from "@opentelemetry/instrumentation-http";

import { NodeSDK } from "@opentelemetry/sdk-node";

import { OTLPMetricExporter as HttpMetricExporer } from "@opentelemetry/exporter-metrics-otlp-http";
import { OTLPMetricExporter as GrpcMetricExporter } from "@opentelemetry/exporter-metrics-otlp-grpc";

const OTLPMetricExporter = config.OTLP_PROTOCOL === "http" ? HttpMetricExporer : GrpcMetricExporter;
//const OTLPTraceExporter = config.OTLP_PROTOCOL === "http" ? HttpTraceExporter : GrpcTraceExporter;

import { Resource } from "@opentelemetry/resources";
import { SemanticResourceAttributes } from "@opentelemetry/semantic-conventions";
import packageInfo from "../package.json";
import { config } from "../config.ts";

const resource = Resource.default().merge(
  new Resource({
    [SemanticResourceAttributes.SERVICE_NAME]: "fiddle-backend",
    [SemanticResourceAttributes.SERVICE_VERSION]: packageInfo.version,
  }),
);

const provider = new NodeTracerProvider({
  resource,
  sampler: new TraceIdRatioBasedSampler(config.TRACE_SAMPLE_RATIO),
});

const metricReader = new PeriodicExportingMetricReader({
  exporter: config.METER_EXPORTER_URL
    ? new OTLPMetricExporter({
        url: config.METER_EXPORTER_URL,
      })
    : new ConsoleMetricExporter({}),
  // collect metrics every 5 seconds
  exportIntervalMillis: 500000,
});
const myServiceMeterProvider = new MeterProvider({
  resource: resource,
});

opentelemetry.metrics.setGlobalMeterProvider(myServiceMeterProvider);
export const meter = myServiceMeterProvider.getMeter("fiddle-backend");

// const traceExporter = config.TRACE_EXPORTER_URL
//   ? new OTLPTraceExporter({ url: config.TRACE_EXPORTER_URL })
//   : new ConsoleSpanExporter();

export const sdk = new NodeSDK({
  resource,
  // @ts-ignore
  metricReader,
});

export const tracer = provider.getTracer("fiddle-backend");
registerInstrumentations({
  instrumentations: [new HttpInstrumentation()],
});
