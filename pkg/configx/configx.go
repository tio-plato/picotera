package configx

import (
	"errors"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	DatabaseURL           string        `mapstructure:"database_url"`
	Host                  string        `mapstructure:"host"`
	Port                  int           `mapstructure:"port"`
	GatewayReadTimeout    time.Duration `mapstructure:"gateway_read_timeout"`
	S3                    S3Config      `mapstructure:"s3"`
	KV                    KVConfig      `mapstructure:"kv"`
	JSHookTimeout         time.Duration `mapstructure:"js_hook_timeout"`
	JSMemoryLimit         int64         `mapstructure:"js_memory_limit"`
	JSMaxTotalAttempts    int           `mapstructure:"js_max_total_attempts"`
	JSMaxDelay            time.Duration `mapstructure:"js_max_delay"`
	LLMBridgeWASMPoolSize int           `mapstructure:"llmbridge_wasm_pool_size"`
	LLMBridgeWASMPath     string        `mapstructure:"llmbridge_wasm_path"`
}

type KVConfig struct {
	Driver   string `mapstructure:"driver"`
	RedisURL string `mapstructure:"redis_url"`
}

type S3Config struct {
	Endpoint  string `mapstructure:"endpoint"`
	Region    string `mapstructure:"region"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	PublicURL string `mapstructure:"public_url"`
}

func Parse() (*Config, error) {
	viper.SetEnvPrefix("PICOTERA")
	viper.AutomaticEnv()

	var config Config
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		var fileLookupError viper.ConfigFileNotFoundError
		if errors.As(err, &fileLookupError) {
			// do nothing
		} else {
			return nil, err
		}
	}

	viper.SetDefault("port", 9898)
	viper.SetDefault("gateway_read_timeout", 300*time.Second)
	viper.SetDefault("s3.region", "us-east-1")
	viper.SetDefault("s3.use_ssl", false)
	viper.SetDefault("js_hook_timeout", 5*time.Second)
	viper.SetDefault("js_memory_limit", int64(64*1024*1024))
	viper.SetDefault("js_max_total_attempts", 50)
	viper.SetDefault("js_max_delay", 60*time.Second)
	viper.SetDefault("kv.driver", "memory")
	viper.SetDefault("kv.redis_url", "localhost:6379")
	viper.SetDefault("llmbridge_wasm_pool_size", runtime.GOMAXPROCS(0))

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	bindEnvs(Config{})
	viper.Unmarshal(&config)
	return &config, nil
}

func bindEnvs(iface interface{}, parts ...string) {
	ifv := reflect.ValueOf(iface)
	ift := reflect.TypeOf(iface)
	for i := 0; i < ift.NumField(); i++ {
		v := ifv.Field(i)
		t := ift.Field(i)
		tv, ok := t.Tag.Lookup("mapstructure")
		if !ok {
			continue
		}
		switch v.Kind() {
		case reflect.Struct:
			bindEnvs(v.Interface(), append(parts, tv)...)
		default:
			viper.BindEnv(strings.Join(append(parts, tv), "."))
		}
	}
}
