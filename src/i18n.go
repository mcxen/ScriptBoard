package main

import (
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
)

// Two languages: 中文 and English. The source text of the UI is Chinese;
// tr maps it to English through the table in i18n_en.go when English is the
// language in use, and returns it as it is otherwise. A string with no entry
// in the table also shows as it is, so a missed string is untranslated, never
// blank.

const (
	langZH = "zh"
	langEN = "en"
)

// lang is the language in use: langZH or langEN.
var lang atomic.Value

func init() { lang.Store(langZH) }

// curLang is the language in use.
func curLang() string { return lang.Load().(string) }

// setLang picks the language: "zh", "en", or "" to follow the system.
func setLang(choice string) {
	switch choice {
	case langZH, langEN:
		lang.Store(choice)
	default:
		lang.Store(systemLang())
	}
}

// systemLang reads the language of the computer: the LANG family of variables,
// then the macOS locale. Anything that is not Chinese is English.
func systemLang() string {
	for _, k := range []string{"SCRIPTBOARD_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := strings.ToLower(os.Getenv(k)); v != "" && v != "c" && v != "posix" {
			if strings.HasPrefix(v, "zh") {
				return langZH
			}
			return langEN
		}
	}
	if out, err := exec.Command("defaults", "read", "-g", "AppleLocale").Output(); err == nil {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(out))), "zh") {
			return langZH
		}
		return langEN
	}
	return langZH
}

// tr translates a UI string.
func tr(s string) string {
	if curLang() == langEN {
		if e, ok := enText[s]; ok {
			return e
		}
	}
	return s
}

// trAll translates a list of strings.
func trAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = tr(s)
	}
	return out
}

// appName is the name of the app in the language in use.
func appName() string {
	if curLang() == langEN {
		return "ScriptBoard"
	}
	return "盯盯板"
}
