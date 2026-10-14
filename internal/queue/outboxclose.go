package queue

import (
	"context"
	"os"
)

// Close останавливает очередь устройства и снимает блокировку. Пустая
// очередь удаляется с диска.
func (o *Outbox) Close() {
	empty := false
	if o.box != nil {
		info, err := o.box.Info(context.Background())
		empty = err == nil && info.State.Msgs == 0
	}
	if o.nc != nil {
		o.nc.Close()
	}
	if o.srv != nil {
		o.srv.Shutdown()
	}
	if empty {
		os.RemoveAll(o.dir)
	}
	o.unlock()
}
