package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	flag "github.com/spf13/pflag"
)

type Config struct {
	SubjectCount uint
	Body         bool
	Quiet        bool
	Agent        string
	LogLevel     string
	LogFile      string
	Pause        string
	InstallAgent string
	Version      bool
	Help         bool
}

func ParseFlags() (*Config, error) {
	cfg := &Config{}

	flags := flag.NewFlagSet("gen-commit-msg", flag.ContinueOnError)
	flags.UintP("subject-count", "n", 5, "number of subject variants")
	flags.Bool("body", true, "generate message body")
	flags.BoolP("quiet", "q", false, "suppress progress output")
	flags.StringP("agent", "a", "gen-commit-msg", "opencode agent name")
	flags.StringP("log-level", "l", "error", "log verbosity")
	flags.String("log-file", "", "log output file, '-' for stdout")
	flags.String("pause", "on-error", "pause before exit: on, off, on-error")
	flags.String("install-agent", "if-not-exists", "agent install behavior: always, if-not-exists, no")
	flags.BoolP("version", "V", false, "print version and exit")
	flags.BoolP("help", "h", false, "print help and exit")

	if err := flags.Parse(os.Args[1:]); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}

	cfg.Version, _ = flags.GetBool("version")
	cfg.Help, _ = flags.GetBool("help")
	if cfg.Version || cfg.Help {
		return cfg, nil
	}

	cfg.SubjectCount = getUintFlagOrEnv(flags, "subject-count", "GCM_SUBJECT_COUNT", 5)
	cfg.Body = getBoolFlagOrEnv(flags, "body", "GCM_BODY", true)
	cfg.Quiet = getBoolFlagOrEnv(flags, "quiet", "GCM_QUIET", false)
	cfg.Agent = getStringFlagOrEnv(flags, "agent", "GCM_AGENT", "gen-commit-msg")
	cfg.LogLevel = getStringFlagOrEnv(flags, "log-level", "GCM_LOG_LEVEL", "error")
	cfg.LogFile = getStringFlagOrEnv(flags, "log-file", "GCM_LOG_FILE", "")
	cfg.Pause = getStringFlagOrEnv(flags, "pause", "GCM_PAUSE", "on-error")
	cfg.InstallAgent = getStringFlagOrEnv(flags, "install-agent", "GCM_INSTALL_AGENT", "if-not-exists")

	return cfg, nil
}

func getStringFlagOrEnv(flags *flag.FlagSet, name, envVar, defaultVal string) string {
	val, _ := flags.GetString(name)
	if flags.Changed(name) {
		return val
	}
	if env := os.Getenv(envVar); env != "" {
		return env
	}
	return defaultVal
}

func getBoolFlagOrEnv(flags *flag.FlagSet, name, envVar string, defaultVal bool) bool {
	val, _ := flags.GetBool(name)
	if flags.Changed(name) {
		return val
	}
	env := os.Getenv(envVar)
	if env == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(env)
	if err != nil {
		slog.Warn("invalid env var value, using default", "env", envVar, "value", env)
		return defaultVal
	}
	return b
}

func Usage() {
	flags := flag.NewFlagSet("gen-commit-msg", flag.ContinueOnError)
	flags.UintP("subject-count", "n", 5, "number of subject variants")
	flags.Bool("body", true, "generate message body")
	flags.BoolP("quiet", "q", false, "suppress progress output")
	flags.StringP("agent", "a", "gen-commit-msg", "opencode agent name")
	flags.StringP("log-level", "l", "error", "log verbosity")
	flags.String("log-file", "", "log output file, '-' for stdout")
	flags.String("pause", "on-error", "pause before exit: on, off, on-error")
	flags.String("install-agent", "if-not-exists", "agent install behavior: always, if-not-exists, no")
	flags.BoolP("version", "V", false, "print version and exit")
	flags.BoolP("help", "h", false, "print help and exit")
	flags.Usage()
}

func getUintFlagOrEnv(flags *flag.FlagSet, name, envVar string, defaultVal uint) uint {
	val, _ := flags.GetUint(name)
	if flags.Changed(name) {
		return val
	}
	env := os.Getenv(envVar)
	if env == "" {
		return defaultVal
	}
	n, err := strconv.ParseUint(env, 10, 64)
	if err != nil {
		slog.Warn("invalid env var value, using default", "env", envVar, "value", env)
		return defaultVal
	}
	return uint(n)
}
