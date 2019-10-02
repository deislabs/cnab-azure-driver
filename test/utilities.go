package test

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

func UnSetDriverEnvironmentVars(t *testing.T) {
	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		if strings.HasPrefix(pair[0], "DUFFLE_ACI_DRIVER") {
			t.Logf("Unsetting Env Variable: %s", pair[0])
			os.Unsetenv(pair[0])
		}

	}
}
func GetFieldValue(t *testing.T, i interface{}, field string) interface{} {
	r := reflect.ValueOf(i)
	f := reflect.Indirect(r).FieldByName(field)
	if f.IsValid() {
		switch f.Kind() {
		case reflect.String:
			return f.String()
		case reflect.Int:
			return f.Int()
		case reflect.Bool:
			return f.Bool()
		default:
			t.Errorf("field %s has unexpected type %s ", field, f.Kind())
			return nil
		}
	}

	t.Errorf("Unable to get value for field %s ", field)
	return nil
}
func SetLoggingLevel(verbose *bool) {
	if *verbose {
		log.SetLevel(log.DebugLevel)
	}
}
func SetStatePathEnvironmentVariables() {
	os.Setenv("DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT", "/cnab/app/state/")
	os.Setenv("DUFFLE_ACI_DRIVER_STATE_PATH", uuid.New().String())
}
