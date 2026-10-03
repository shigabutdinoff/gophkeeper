package cli

import (
	"cmp"
	"io"

	"github.com/spf13/cobra"
)

func init() {
	cobra.MousetrapHelpText = "Это консольная программа.\n\nЗапустите её из командной строки cmd.exe или PowerShell.\n"
}

// New возвращает корневую команду gophkeeper. Её флаг version печатает
// версию, номер, дату и коммит сборки, пустое значение как N/A. Результаты
// идут в out, сообщения cobra в errOut, а ошибка возвращается из Execute.
func New(version, number, date, commit string, out, errOut io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:     "gophkeeper",
		Short:   "Клиент менеджера паролей GophKeeper",
		Version: cmp.Or(version, "N/A"),
		Annotations: map[string]string{
			"number": cmp.Or(number, "N/A"),
			"date":   cmp.Or(date, "N/A"),
			"commit": cmp.Or(commit, "N/A"),
		},
		CompletionOptions: cobra.CompletionOptions{DisableNoDescFlag: true},
	}
	root.CompletionOptions.SetDefaultShellCompDirective(cobra.ShellCompDirectiveNoFileComp)
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetVersionTemplate("Build version: {{.Version}}\nBuild number: {{.Annotations.number}}\nBuild date: {{.Annotations.date}}\nBuild commit: {{.Annotations.commit}}\n")
	root.InitDefaultVersionFlag()
	root.Flags().Lookup("version").Usage = "показать версию, номер, дату и коммит сборки"
	root.InitDefaultCompletionCmd("completion")
	Localize(root)
	return root
}
