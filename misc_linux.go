package systemdutil

import (
	"os"

	"github.com/coreos/go-systemd/v22/activation"
)

func ActivationFiles() []*os.File {
	return activation.Files(true)
}
