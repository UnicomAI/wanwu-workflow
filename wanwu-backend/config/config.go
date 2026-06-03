package config

import (
	"os"

	"github.com/UnicomAI/wanwu/pkg/db"
	"github.com/UnicomAI/wanwu/pkg/log"
	"github.com/UnicomAI/wanwu/pkg/util"
	coze_workflow_config "github.com/coze-dev/coze-studio/backend/domain/workflow/config"

	"github.com/UnicomAI/wanwu-workflow/wanwu-backend/pkg/redis"
)

var (
	_c *Config
)

type Config struct {
	Server ServerConfig `json:"server" mapstructure:"server"`
	Log    LogConfig    `json:"log" mapstructure:"log"`
	JWT    JWTConfig    `json:"jwt" mapstructure:"jwt"`
	DB     db.Config    `json:"db" mapstructure:"db"`
	Redis  redis.Config `json:"redis" mapstructure:"redis"`
	Minio  MinioConfig  `json:"minio" mapstructure:"minio"`
	Icons  []IconConfig `json:"icons" mapstructure:"icons"`

	Workflow *coze_workflow_config.WorkflowConfig `json:"workflow" mapstructure:"workflow"`
}

type ServerConfig struct {
	HttpEndpoint   string `json:"http_endpoint" mapstructure:"http_endpoint"`
	MaxReqBodySize int    `json:"max_req_body_size" mapstructure:"max_req_body_size"`
}

type LogConfig struct {
	Std   bool         `json:"std" mapstructure:"std"`
	Level string       `json:"level" mapstructure:"level"`
	Logs  []log.Config `json:"logs" mapstructure:"logs"`
}

type JWTConfig struct {
	SigningKey string `json:"signing-key" mapstructure:"signing-key"`
}

type MinioConfig struct {
	Endpoint string `json:"endpoint" mapstructure:"endpoint"`
	User     string `json:"user" mapstructure:"user"`
	Password string `json:"password" mapstructure:"password"`
}

type IconConfig struct {
	ID  string `json:"id" mapstructure:"id"`
	Url string `json:"url" mapstructure:"url"`
}

func LoadConfig(in string) error {
	_c = &Config{}
	if err := util.LoadConfig(in, _c); err != nil {
		return err
	}
	for _, icon := range _c.Icons {
		switch icon.ID {
		case "9":
			// 工作流默认图标
			if err := os.Setenv("WANWU_WORKFLOW_DEFAULT_ICON", icon.Url); err != nil {
				return err
			}
		case "-9":
			// 对话流默认图标
			if err := os.Setenv("WANWU_CHATFLOW_DEFAULT_ICON", icon.Url); err != nil {
				return err
			}
		}
	}
	return nil
}

func Cfg() *Config {
	if _c == nil {
		log.Panicf("cfg nil")
	}
	return _c
}
