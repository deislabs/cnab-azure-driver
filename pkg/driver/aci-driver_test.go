package driver

import (
	"testing"

	cnabdriver "github.com/deislabs/cnab-go/driver"
	"github.com/stretchr/testify/assert"
)

func TestHandles(t *testing.T) {
	d := &ACIDriver{}

	assert.Equal(t, true, d.Handles(cnabdriver.ImageTypeDocker))
	assert.Equal(t, true, d.Handles(cnabdriver.ImageTypeOCI))
	assert.Equal(t, false, d.Handles(cnabdriver.ImageTypeQCOW))

}
