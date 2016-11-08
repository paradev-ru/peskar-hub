package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var (
	replacer = strings.NewReplacer("/", "_")
)

type Client struct {
	DataDir string
}

func NewBackend(datadir string) *Client {
	return &Client{datadir}
}

func transform(key string) string {
	k := strings.TrimPrefix(key, "/")
	return strings.ToLower(replacer.Replace(k))
}

func (c *Client) Load(key string, vars interface{}) error {
	filename := filepath.Join(c.DataDir, fmt.Sprintf("%s.json", transform(key)))
	if _, err := os.Stat(filename); err != nil {
		return err
	}
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = json.Unmarshal(file, &vars)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Save(key string, vars interface{}) error {
	filename := filepath.Join(c.DataDir, fmt.Sprintf("%s.json", transform(key)))
	keyFile, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("Could not open file: %v", err)
	}
	defer keyFile.Close()
	json, err := json.Marshal(vars)
	if err != nil {
		return err
	}
	_, err = keyFile.WriteString(string(json))
	if err != nil {
		return fmt.Errorf("Could not write to file: %s", err)
	}
	return nil
}
