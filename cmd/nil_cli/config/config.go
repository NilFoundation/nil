package config

type Config struct {
	RPCEndpoint string `mapstructure:"rpc_endpoint"`
	PrivateKey  string `mapstructure:"private_key"`
}
