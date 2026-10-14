package snapstest

import (
	"log"
	"testing"
)

func TestMain(m *testing.M) {
	if (CleanM{m}).Run() != 0 {
		log.Fatal("тесты пакета или снапшоты не прошли проверку")
	}
}
