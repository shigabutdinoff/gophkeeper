package main

import (
	"cmp"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/urfave/sflags"
	"github.com/urfave/sflags/gen/gpflag"

	"github.com/shigabutdinoff/gophkeeper/internal/config"
)

func addFlags(fs *pflag.FlagSet, s any) []*sflags.Flag {
	fs.StringP("config", "c", "", "`файл` настроек YAML или JSON, если не задана CONFIG")
	flags, _ := sflags.ParseStruct(s, sflags.EnvPrefix("GOPHKEEPER_"))
	gpflag.GenerateTo(flags, fs)
	for _, f := range flags {
		fs.Lookup(f.Name).Usage = "`" + strings.Replace(f.Usage, " ", "` ", 1) + ", если не задана " + f.EnvNames[0] + ", важнее файла настроек"
	}
	return flags
}

func load(fs *pflag.FlagSet, flags []*sflags.Flag) error {
	path, _ := fs.GetString("config")
	file, err := config.Load[map[string]string](cmp.Or(os.Getenv("CONFIG"), path))
	if err != nil {
		return err
	}
	var lacks []string
	for _, f := range flags {
		key := strings.ReplaceAll(f.Name, "-", "_")
		_ = f.Value.Set(cmp.Or(os.Getenv(f.EnvNames[0]), f.Value.String(), file[key]))
		if f.Required && f.Value.String() == "" {
			lacks = append(lacks, fmt.Sprintf("%s (%s, --%s или %s в файле настроек)", f.Usage, f.EnvNames[0], f.Name, key))
		}
	}
	if lacks != nil {
		return fmt.Errorf("не заданы %s", strings.Join(lacks, ", "))
	}
	return nil
}
