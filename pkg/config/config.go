package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

// Config holds all configuration values
type Config struct {
	Hostname      string `toml:"hostname"`
	MQTTHost      string `toml:"mqtt_host"`
	MQTTPort      int    `toml:"mqtt_port"`
	MQTTUsername  string `toml:"mqtt_username"`
	MQTTPassword  string `toml:"mqtt_password"`
	MQTTLWTEnable bool   `toml:"mqtt_lwt_enable"`
	MQTTRetain    bool   `toml:"mqtt_retain"`
	PollingPeriod int    `toml:"polling_period"`
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	return &Config{
		Hostname:      "",
		MQTTHost:      "localhost",
		MQTTPort:      1883,
		MQTTUsername:  "",
		MQTTPassword:  "",
		MQTTLWTEnable: true,
		MQTTRetain:    true,
		PollingPeriod: 30,
	}
}

// LoadConfigFromFile loads configuration from TOML file
func LoadConfigFromFile(filename string) (*Config, error) {
	config := DefaultConfig()

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// File doesn't exist, return default config
		return config, nil
	}

	// Read and parse TOML file
	if _, err := toml.DecodeFile(filename, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %v", filename, err)
	}

	return config, nil
}

// LoadConfig loads configuration from file first, then overrides with command line flags
func LoadConfig(cmd *cobra.Command) (*Config, error) {
	// First load from config file
	configFile := "/etc/nvml-gpu-ha.conf"

	// Allow override of config file path via flag
	if cmd.Flags().Changed("config") {
		var err error
		configFile, err = cmd.Flags().GetString("config")
		if err != nil {
			return nil, err
		}
	}

	config, err := LoadConfigFromFile(configFile)
	if err != nil {
		return nil, err
	}

	// Override with command line flags if they were explicitly set
	if cmd.Flags().Changed("hostname") {
		config.Hostname, err = cmd.Flags().GetString("hostname")
		if err != nil {
			return nil, err
		}
	}

	if cmd.Flags().Changed("mqtt-host") {
		config.MQTTHost, err = cmd.Flags().GetString("mqtt-host")
		if err != nil {
			return nil, err
		}
	}

	if cmd.Flags().Changed("mqtt-port") {
		config.MQTTPort, err = cmd.Flags().GetInt("mqtt-port")
		if err != nil {
			return nil, err
		}
	}

	if cmd.Flags().Changed("mqtt-username") {
		config.MQTTUsername, err = cmd.Flags().GetString("mqtt-username")
		if err != nil {
			return nil, err
		}
	}

	if cmd.Flags().Changed("mqtt-password") {
		config.MQTTPassword, err = cmd.Flags().GetString("mqtt-password")
		if err != nil {
			return nil, err
		}
	}

	if cmd.Flags().Changed("mqtt-lwt-enable") {
		config.MQTTLWTEnable, err = cmd.Flags().GetBool("mqtt-lwt-enable")
		if err != nil {
			return nil, err
		}
	}

	if cmd.Flags().Changed("mqtt-retain") {
		config.MQTTRetain, err = cmd.Flags().GetBool("mqtt-retain")
		if err != nil {
			return nil, err
		}
	}

	if cmd.Flags().Changed("polling-period") {
		config.PollingPeriod, err = cmd.Flags().GetInt("polling-period")
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}

// SaveToFile saves current configuration to a TOML file
func (c *Config) SaveToFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create config file %s: %v", filename, err)
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to encode config to file %s: %v", filename, err)
	}

	return nil
}
