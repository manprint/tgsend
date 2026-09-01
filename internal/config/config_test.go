package config

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/manprint/tgsend/internal/apperr"
)

func TestLoadTOMLStringChatID(t *testing.T) {
	config, err := loadWith(`token = "file-token"
chat_id = "@team_room"
`)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.Token != "file-token" || config.ChatID != "@team_room" {
		t.Fatalf("Load() = %#v", config)
	}
}

func TestLoadTOMLIntegerChatID(t *testing.T) {
	config, err := loadWith(`token = "file-token"
chat_id = -100123
`)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.ChatID != "-100123" {
		t.Fatalf("ChatID = %q, want -100123", config.ChatID)
	}
}

func TestEnvironmentOverridesEachField(t *testing.T) {
	config, err := Load(LoadOptions{
		ReadFile: func(string) ([]byte, error) {
			return []byte("token = \"file-token\"\nchat_id = \"-100\"\n"), nil
		},
		LookupEnv: env(map[string]string{
			tokenEnv:  "env-token",
			chatIDEnv: "@env_chat",
		}),
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config != (Config{Token: "env-token", ChatID: "@env_chat"}) {
		t.Fatalf("Load() = %#v", config)
	}
}

func TestEnvironmentCompletesMissingDefault(t *testing.T) {
	config, err := Load(LoadOptions{
		HomeDir: func() (string, error) { return "/safe/home", nil },
		ReadFile: func(path string) ([]byte, error) {
			if path != "/safe/home/.tgsend" {
				t.Fatalf("ReadFile path = %q", path)
			}
			return nil, fs.ErrNotExist
		},
		LookupEnv: env(map[string]string{tokenEnv: "env-token", chatIDEnv: "-100"}),
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config != (Config{Token: "env-token", ChatID: "-100"}) {
		t.Fatalf("Load() = %#v", config)
	}
}

func TestExplicitMissingFailsWithCompleteEnvironment(t *testing.T) {
	_, err := Load(LoadOptions{
		Path:     "/safe/missing.toml",
		Explicit: true,
		ReadFile: func(string) ([]byte, error) { return nil, fs.ErrNotExist },
		LookupEnv: env(map[string]string{
			tokenEnv:  "env-token",
			chatIDEnv: "-100",
		}),
	})
	assertCode(t, err, apperr.CodeConfigNotFound)
}

func TestMalformedExistingFileWinsOverEnvironment(t *testing.T) {
	_, err := Load(LoadOptions{
		ReadFile: func(string) ([]byte, error) { return []byte("token = ["), nil },
		LookupEnv: env(map[string]string{
			tokenEnv:  "env-token",
			chatIDEnv: "-100",
		}),
	})
	assertCode(t, err, apperr.CodeConfigInvalid)
}

func TestUnknownKeyRejected(t *testing.T) {
	_, err := loadWith(`token = "file-token"
chat_id = "-100"
unknown = "reject"
`)
	assertCode(t, err, apperr.CodeConfigInvalid)
}

func TestWrongTypesRejected(t *testing.T) {
	tests := []string{
		"token = 123\nchat_id = \"-100\"\n",
		"token = \"token\"\nchat_id = [1, 2]\n",
	}
	for _, contents := range tests {
		t.Run(contents, func(t *testing.T) {
			_, err := loadWith(contents)
			assertCode(t, err, apperr.CodeConfigInvalid)
		})
	}
}

func TestMissingTokenAndChatClassified(t *testing.T) {
	_, err := loadWith("")
	assertCode(t, err, apperr.CodeConfigIncomplete)
}

func TestInvalidChatIDs(t *testing.T) {
	tests := []string{"0", "-0", "12 3", "+123", "123x", "@", "@bad-name", "@bad space", "9223372036854775808", "-9223372036854775809", "１"}
	for _, chatID := range tests {
		t.Run(chatID, func(t *testing.T) {
			_, err := loadWith(fmt.Sprintf("token = %q\nchat_id = %q\n", "token", chatID))
			assertCode(t, err, apperr.CodeConfigInvalid)
		})
	}
}

func TestReadErrorClassified(t *testing.T) {
	_, err := Load(LoadOptions{
		ReadFile:  func(string) ([]byte, error) { return nil, errors.New("token=secret-low-level-error") },
		LookupEnv: env(map[string]string{tokenEnv: "secret-env-token", chatIDEnv: "-100"}),
	})
	assertCode(t, err, apperr.CodeConfigUnreadable)
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked secret: %v", err)
	}
}

func TestErrorsRedactToken(t *testing.T) {
	const token = "super-secret-token"
	tests := []LoadOptions{
		{
			ReadFile:  func(string) ([]byte, error) { return []byte("token = \"" + token + "\"\nchat_id = \"bad id\"\n"), nil },
			LookupEnv: env(map[string]string{}),
		},
		{
			ReadFile:  func(string) ([]byte, error) { return nil, errors.New("low-level " + token) },
			LookupEnv: env(map[string]string{tokenEnv: token}),
		},
	}
	for _, options := range tests {
		_, err := Load(options)
		if err == nil || strings.Contains(err.Error(), token) {
			t.Fatalf("Load() error leaked token: %v", err)
		}
	}
}

func FuzzLoadNeverEchoesToken(f *testing.F) {
	f.Add("seed-token")
	f.Add("token with spaces")
	f.Fuzz(func(t *testing.T, token string) {
		if token == "" {
			t.Skip()
		}
		_, err := Load(LoadOptions{
			ReadFile: func(string) ([]byte, error) {
				return []byte("token = \"unterminated"), errors.New(token)
			},
			LookupEnv: env(map[string]string{tokenEnv: token}),
		})
		if err != nil && strings.Contains(err.Error(), token) {
			t.Fatalf("Load() error leaked token: %v", err)
		}
	})
}

func loadWith(contents string) (Config, error) {
	return Load(LoadOptions{
		ReadFile:  func(string) ([]byte, error) { return []byte(contents), nil },
		LookupEnv: env(map[string]string{}),
	})
}

func env(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func assertCode(t *testing.T, err error, want apperr.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", want)
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if appErr.Code != want {
		t.Fatalf("error code = %s, want %s", appErr.Code, want)
	}
}
