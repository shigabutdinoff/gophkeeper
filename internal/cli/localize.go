package cli

import (
	_ "embed"

	"github.com/spf13/cobra"
)

//go:embed usage.tmpl
var usageTemplate string

// Localize переводит на русский справку, help, автодополнение и ошибки
// флагов root, уже собранной с подкомандами. Без подкоманды root печатает
// справку, а неизвестную отклоняет.
func Localize(root *cobra.Command) {
	root.Args, root.RunE = noSubcommand, showHelp
	root.SilenceErrors, root.SilenceUsage = true, true
	root.SetUsageTemplate(usageTemplate)
	root.SetFlagErrorFunc(flagError)
	root.PersistentFlags().BoolP("help", "h", false, "показать справку по команде")
	root.PersistentFlags().Lookup("help").Annotations = map[string][]string{cobra.FlagSetByCobraAnnotation: {"true"}}
	root.InitDefaultHelpCmd()
	for _, cmd := range root.Commands() {
		switch cmd.Name() {
		case "completion":
			localizeCompletionCmd(cmd)
		case "help":
			localizeHelpCmd(cmd)
		}
	}
}
