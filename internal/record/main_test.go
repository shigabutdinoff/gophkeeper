package record

import (
	"log"
	"testing"

	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

func TestMain(m *testing.M) {
	if (snapstest.CleanM{M: m}).Run() != 0 {
		log.Fatal("тесты пакета или снапшоты не прошли проверку")
	}
}
