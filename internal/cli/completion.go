package cli

import "github.com/spf13/cobra"

func localizeCompletionCmd(completion *cobra.Command) {
	load := map[string]string{
		"bash":       "source <(gophkeeper completion bash)",
		"zsh":        "source <(gophkeeper completion zsh)",
		"fish":       "gophkeeper completion fish | source",
		"powershell": "gophkeeper completion powershell | Out-String | Invoke-Expression",
	}
	completion.Short = "Вывести скрипт автодополнения для оболочки"
	completion.Long = "Выводит скрипт автодополнения команд gophkeeper для bash, zsh, fish или powershell.\n" +
		"Как подключить скрипт, сказано в справке каждой оболочки."
	completion.Args = noSubcommand
	completion.RunE = showHelp
	for _, shell := range completion.Commands() {
		shell.Short = "Скрипт автодополнения для " + shell.Name()
		shell.Long = "Выводит скрипт автодополнения для " + shell.Name() + ". Подключить в текущем сеансе:\n\n  " + load[shell.Name()]
		shell.Args = NoArgs
	}
}
