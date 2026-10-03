package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	log.SetFlags(0)
	if err := run(os.Args[1:]); err != nil {
		log.Fatal("Ошибка: ", err)
	}
}

func run(args []string) error {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("неизвестный флаг %q", arg)
		}
	}
	if len(args) != 2 {
		return errors.New("нужны два аргумента, файл и версия")
	}
	number, err := compute(args[1])
	if err != nil {
		return err
	}
	if err := os.WriteFile(args[0], []byte(number+"\n"), 0o644); err != nil {
		return fmt.Errorf("запись номера сборки: %w", err)
	}
	return nil
}
