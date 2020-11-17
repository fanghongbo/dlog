package dlog

type multiBackend struct {
	bes []Backend
}

func NewMultiBackend(bes ...Backend) (*multiBackend, error) {
	var b multiBackend
	b.bes = bes
	return &b, nil
}

func (u *multiBackend) Log(s Severity, msg []byte) {
	for _, be := range u.bes {
		be.Log(s, msg)
	}
}

func (u *multiBackend) close() {
	for _, be := range u.bes {
		be.close()
	}
}
