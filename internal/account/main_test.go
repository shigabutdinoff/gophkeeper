package account

import (
	"log"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/shigabutdinoff/gophkeeper/internal/snapstest"
)

func TestMain(m *testing.M) {
	keyring.MockInit()
	if (snapstest.CleanM{M: m}).Run() != 0 {
		log.Fatal("тесты пакета или снапшоты не прошли проверку")
	}
}
