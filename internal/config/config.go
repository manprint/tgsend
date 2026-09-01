package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/manprint/tgsend/internal/apperr"
	"github.com/pelletier/go-toml/v2"
)

const (
	tokenEnv  = "TGSEND_TOKEN"
	chatIDEnv = "TGSEND_CHAT_ID"
)

// Config contains the credentials and destination needed for a send.
type Config struct {
	Token  string
	ChatID string
}

// LoadOptions makes configuration I/O injectable and keeps loading deterministic in tests.
type LoadOptions struct {
	Path      string
	Explicit  bool
	HomeDir   func() (string, error)
	ReadFile  func(string) ([]byte, error)
	LookupEnv func(string) (string, bool)
}

type rawConfig struct {
	Token  string `toml:"token"`
	ChatID any    `toml:"chat_id"`
}

// Load reads the selected TOML file and applies non-empty environment overrides.
// The default file is optional; an explicitly selected file is never optional.
func Load(options LoadOptions) (Config, error) {
	readFile := options.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	lookupEnv := options.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}

	path := options.Path
	if options.Explicit && path == "" {
		return Config{}, invalid("an explicit configuration path is required", nil)
	}
	if path == "" {
		homeDir := options.HomeDir
		if homeDir == nil {
			homeDir = os.UserHomeDir
		}
		home, err := homeDir()
		if err != nil || home == "" {
			return Config{}, unreadable("configuration home is unavailable", err)
		}
		path = filepath.Join(home, ".tgsend")
	}

	data, err := readFile(path)
	if err != nil {
		if !options.Explicit && errors.Is(err, fs.ErrNotExist) {
			data = nil
		} else if errors.Is(err, fs.ErrNotExist) {
			return Config{}, notFound("configuration file not found", err)
		} else {
			return Config{}, unreadable("configuration file is not readable", err)
		}
	}

	var raw rawConfig
	if len(data) > 0 {
		decoder := toml.NewDecoder(strings.NewReader(string(data))).DisallowUnknownFields()
		if err := decoder.Decode(&raw); err != nil {
			return Config{}, invalid("configuration is invalid", err)
		}
	}

	chatID, err := normalizeChatID(raw.ChatID)
	if err != nil {
		return Config{}, invalid("configuration is invalid", err)
	}

	token, tokenSet := lookupEnv(tokenEnv)
	if !tokenSet || token == "" {
		token = raw.Token
	}
	envChatID, chatIDSet := lookupEnv(chatIDEnv)
	if !chatIDSet || envChatID == "" {
		envChatID = chatID
	}

	if token == "" || envChatID == "" {
		return Config{}, incomplete("configuration is incomplete", nil)
	}
	if !validChatID(envChatID) {
		return Config{}, invalid("configuration is invalid", nil)
	}

	return Config{Token: token, ChatID: envChatID}, nil
}

func normalizeChatID(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	switch typed := value.(type) {
	case string:
		return typed, nil
	case int:
		return strconv.FormatInt(int64(typed), 10), nil
	case int8, int16, int32, int64:
		return strconv.FormatInt(reflect.ValueOf(typed).Int(), 10), nil
	default:
		return "", errors.New("chat_id has an unsupported type")
	}
}

func validChatID(value string) bool {
	if value == "" || strings.IndexFunc(value, func(r rune) bool { return r > unicodeMaxASCII }) >= 0 {
		return false
	}
	if strings.HasPrefix(value, "@") {
		if len(value) == 1 {
			return false
		}
		for _, char := range value[1:] {
			if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' {
				return false
			}
		}
		return true
	}

	if value[0] == '-' {
		if len(value) == 1 {
			return false
		}
	} else if value[0] < '0' || value[0] > '9' {
		return false
	}
	start := 0
	if value[0] == '-' {
		start = 1
	}
	for _, char := range value[start:] {
		if char < '0' || char > '9' {
			return false
		}
	}
	number, err := strconv.ParseInt(value, 10, 64)
	return err == nil && number != 0
}

const unicodeMaxASCII = 127

func notFound(message string, cause error) *apperr.Error {
	return apperr.New(apperr.KindConfig, apperr.CodeConfigNotFound, message, cause)
}

func unreadable(message string, cause error) *apperr.Error {
	return apperr.New(apperr.KindConfig, apperr.CodeConfigUnreadable, message, cause)
}

func invalid(message string, cause error) *apperr.Error {
	return apperr.New(apperr.KindConfig, apperr.CodeConfigInvalid, message, cause)
}

func incomplete(message string, cause error) *apperr.Error {
	return apperr.New(apperr.KindConfig, apperr.CodeConfigIncomplete, message, cause)
}
