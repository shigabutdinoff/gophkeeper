package snapstest

import (
	"os"

	"github.com/gkampitakis/ciinfo"
)

// UpdateScripts сообщает, нужно ли переписать эталоны testscript. Эталоны
// переписываются при UPDATE_SNAPS=true вне CI.
func UpdateScripts() bool {
	return os.Getenv("UPDATE_SNAPS") == "true" && !ciinfo.IsCI
}
