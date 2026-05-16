package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	flag "github.com/spf13/pflag"
)

type Config struct {
	SubjectMin   uint
	SubjectMax   uint
	Body         bool
	Quiet        bool
	Agent        string
	LogLevel     string
	LogFile      string
	Pause        string
	InstallAgent string
	Output       string
	Version      bool
	Help         bool
}

func initFlags() *flag.FlagSet {
	flags := flag.NewFlagSet("gen-commit-msg", flag.ContinueOnError)
	flags.UintP("subject-min", "m", 1, "minimum number of subject variants")
	flags.UintP("subject-max", "x", 5, "maximum number of subject variants")
	flags.Bool("body", true, "generate message body")
	flags.BoolP("quiet", "q", false, "suppress progress output")
	flags.StringP("agent", "a", "gen-commit-msg", "opencode agent name")
	flags.StringP("log-level", "l", "none", "log verbosity: trace, debug, info, warn, error, none")
	flags.String("log-file", "", "log output file (default: stderr)")
	flags.String("pause", "on-error", "pause before exit: on, off, on-error")
	flags.String("install-agent", "if-not-exists", "agent install behavior: always, if-not-exists, no")
	flags.StringP("output", "o", "", "write commit message to file instead of stdout")
	flags.BoolP("version", "V", false, "print version and exit")
	flags.BoolP("help", "h", false, "print help and exit")
	return flags
}

func ParseFlags() (*Config, error) {
	cfg := &Config{}

	flags := initFlags()

	if err := flags.Parse(os.Args[1:]); err != nil {
		slog.Error("failed to parse flags", "error", err)
		return nil, fmt.Errorf("parse flags: %w", err)
	}

	cfg.Version, _ = flags.GetBool("version")
	cfg.Help, _ = flags.GetBool("help")
	if cfg.Version || cfg.Help {
		cfg.LogLevel = "none"
		slog.Debug("version or help flag set, skipping config resolution")
		return cfg, nil
	}

	cfg.SubjectMin = getUintFlagOrEnv(flags, "subject-min", "GCM_SUBJECT_MIN", 1)
	cfg.SubjectMax = getUintFlagOrEnv(flags, "subject-max", "GCM_SUBJECT_MAX", 5)
	cfg.Body = getBoolFlagOrEnv(flags, "body", "GCM_BODY", true)
	cfg.Quiet = getBoolFlagOrEnv(flags, "quiet", "GCM_QUIET", false)
	cfg.Agent = getStringFlagOrEnv(flags, "agent", "GCM_AGENT", "gen-commit-msg")
	cfg.LogLevel = getStringFlagOrEnv(flags, "log-level", "GCM_LOG_LEVEL", "none")
	cfg.LogFile = getStringFlagOrEnv(flags, "log-file", "GCM_LOG_FILE", "")
	cfg.Pause = getStringFlagOrEnv(flags, "pause", "GCM_PAUSE", "on-error")
	cfg.InstallAgent = getStringFlagOrEnv(flags, "install-agent", "GCM_INSTALL_AGENT", "if-not-exists")
	cfg.Output = getStringFlagOrEnv(flags, "output", "GCM_OUTPUT", "")

	if cfg.SubjectMin < 1 {
		return nil, fmt.Errorf("subject-min must be at least 1, got %d", cfg.SubjectMin)
	}
	if cfg.SubjectMax > 20 {
		return nil, fmt.Errorf("subject-max must not exceed 20, got %d", cfg.SubjectMax)
	}
	if cfg.SubjectMax < cfg.SubjectMin {
		maxSetExplicitly := flags.Changed("subject-max") || os.Getenv("GCM_SUBJECT_MAX") != ""
		if !maxSetExplicitly {
			cfg.SubjectMax = cfg.SubjectMin
		} else {
			return nil, fmt.Errorf("subject-max (%d) must be >= subject-min (%d)", cfg.SubjectMax, cfg.SubjectMin)
		}
	}

	return cfg, nil
}

func getStringFlagOrEnv(flags *flag.FlagSet, name, envVar, defaultVal string) string {
	val, _ := flags.GetString(name)
	if flags.Changed(name) {
		slog.Debug("config resolved from flag", "name", name, "value", val)
		return val
	}
	if env := os.Getenv(envVar); env != "" {
		slog.Debug("config resolved from env", "name", name, "env", envVar, "value", env)
		return env
	}
	slog.Debug("config resolved from default", "name", name, "value", defaultVal)
	return defaultVal
}

func getBoolFlagOrEnv(flags *flag.FlagSet, name, envVar string, defaultVal bool) bool {
	val, _ := flags.GetBool(name)
	if flags.Changed(name) {
		slog.Debug("config resolved from flag", "name", name, "value", val)
		return val
	}
	env := os.Getenv(envVar)
	if env == "" {
		slog.Debug("config resolved from default", "name", name, "value", defaultVal)
		return defaultVal
	}
	b, err := strconv.ParseBool(env)
	if err != nil {
		slog.Warn("invalid env var value, using default", "env", envVar, "value", env)
		return defaultVal
	}
	slog.Debug("config resolved from env", "name", name, "env", envVar, "value", b)
	return b
}

func Usage() {
	flags := initFlags()
	flags.SetOutput(os.Stdout)
	flags.Usage = func() {
		_, _ = fmt.Fprintf(flags.Output(), "Usage of %s:\n", flags.Name())
		flags.PrintDefaults()
	}
	flags.Usage()
}

func getUintFlagOrEnv(flags *flag.FlagSet, name, envVar string, defaultVal uint) uint {
	val, _ := flags.GetUint(name)
	if flags.Changed(name) {
		slog.Debug("config resolved from flag", "name", name, "value", val)
		return val
	}
	env := os.Getenv(envVar)
	if env == "" {
		slog.Debug("config resolved from default", "name", name, "value", defaultVal)
		return defaultVal
	}
	n, err := strconv.ParseUint(env, 10, 64)
	if err != nil {
		slog.Warn("invalid env var value, using default", "env", envVar, "value", env)
		return defaultVal
	}
	slog.Debug("config resolved from env", "name", name, "env", envVar, "value", n)
	return uint(n)
}

func (c *Config) ValidateOutputPath() error {
	if c.Output == "" {
		return nil
	}
	dir := filepath.Dir(c.Output)
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("failed to open output file %q: no such file or directory", c.Output)
		}
		return fmt.Errorf("failed to open output file %q: %w", c.Output, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("failed to open output file %q: not a directory", c.Output)
	}
	f, err := os.OpenFile(c.Output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open output file %q: %w", c.Output, err)
	}
	_ = f.Close()
	_ = os.Remove(c.Output)
	return nil
}
