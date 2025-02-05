package config

import "os"
import "path/filepath"
import "encoding/json"
import "fmt"

type Config struct {
	DbUrl string `json:"db_url"`
	CurrentUsername string `json:"current_user_name"`
}

func getConfigFilepath() (string, error) {
	homeDirectoryPath, err := os.UserHomeDir() // (string, error)
	if err != nil {
		fmt.Printf("os.UserHomeDir() errored: %s\n", err)
		return "", err
	}
	filePath := filepath.Join(homeDirectoryPath, ".gatorconfig.json")
	return filePath, nil
}

func Read() (Config, error) {
	var config Config
	filePath, err := getConfigFilepath() // discard error
	if err != nil {
		fmt.Printf("getConfigFilepath() errored: %s\n", err)
		return config, err
	}
	fileContent, err := os.ReadFile(filePath);
	if err != nil {
		fmt.Printf("os.ReadFile() errored: %s\n", err)
		return config, err
	}

	err = json.Unmarshal(fileContent, &config)
	if err != nil {
		fmt.Printf("json.Unmarshal() errored: %s\n", err)
	}
	return config, err
}

func write(c Config) error {
	fileContent, err := json.Marshal(c)  // []byte, error
	if err != nil {
		fmt.Printf("json.Marshal() errored: %s\n", err)
		return err
	}

	filePath, err := getConfigFilepath() // discard error
	if err != nil {
		fmt.Printf("getConfigFilepath() errored: %s\n", err)
		return err
	}

	file, err := os.Create(filePath) // (*File, error)
	if err != nil {
		fmt.Printf("os.Create() errored: %s\n", err)
		return err
	}

	n, err := file.Write(fileContent)
	if err != nil {
		fmt.Printf("file.Write() errored: %s\n", err)
		return err
	}
	fmt.Printf("Config file: %d bytes written at %s\n", n, filePath)
	return nil
}

func (c *Config) SetUser(user string) error {
	c.CurrentUsername = user
	err := write(*c)
	if err != nil {
		fmt.Printf("setUser() errored: %s\n", err)
	}
  return err
}
