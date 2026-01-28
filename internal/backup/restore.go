package backup

import "context"

type RestoreManager struct {
	Options interface{}
}

func (m *RestoreManager) Run(ctx context.Context) error {
	return nil
}
