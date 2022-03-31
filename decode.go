package confdecoder

import (
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
)

func DecodeFile(filepath string, v interface{}) error {
	rawdata, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}

	data := make(map[string]interface{})

	lines := strings.Split(string(rawdata), "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		elems := strings.Split(strings.TrimSpace(line), " ")
		if len(elems) == 1 {
			continue
		}
		if data[elems[0]] == nil {
			if len(elems) > 2 {
				valueList := make([]string, 0, len(elems)-1)
				for i := 1; i < len(elems); i++ {
					if len(elems[i]) != 0 {
						valueList = append(valueList, elems[i])
					}
				}
				data[elems[0]] = valueList
			} else {
				data[elems[0]] = elems[1]
			}
		} else {
			return errors.New("two lines with the same name")
		}
	}
	if len(data) == 0 {
		return nil // no err on empty file
	}

	pv := reflect.ValueOf(v)
	if pv.Kind() != reflect.Ptr {
		return errors.New("arg must be a pointer")
	}
	if pv.IsNil() {
		return errors.New("arg is nil")
	}
	sv := pv.Elem()
	if sv.Kind() != reflect.Struct {
		return errors.New("arg must be a pointer to a struct")
	}
	if sv.NumField() == 0 {
		return nil // no err on empty struct
	}

	for key, val := range data {
		fv := sv.FieldByName(key)
		vv := reflect.ValueOf(val)

		if fv.IsValid() && fv.CanSet() {
			switch fv.Kind() {
			case reflect.String:
				if vv.Kind() == reflect.String {
					fv.SetString(vv.String())
				} else {
					return errors.New("cant set value of type " + vv.Kind().String() + " to field named " + key)
				}
			case reflect.Int:
				if vv.Kind() == reflect.String {
					if convint, err := strconv.Atoi(vv.String()); err == nil {
						covint64 := int64(convint)
						if fv.OverflowInt(covint64) {
							return errors.New("value of field " + key + " overflows int")
						}
						fv.SetInt(covint64)
					} else {
						return errors.New("cant convert value of field " + key + " to int, err: " + err.Error())
					}
				} else {
					return errors.New("cant set value of type slice to field named " + key)
				}
			case reflect.Slice:
				switch fv.Type().Elem().Kind() {
				case reflect.String:
					if vv.Kind() == reflect.String {
						fv.Set(reflect.MakeSlice(fv.Type(), 1, 1))
						fv.Index(0).Set(vv)
						continue
					}
					fv.Set(reflect.MakeSlice(fv.Type(), vv.Len(), vv.Len()))
					for i := 0; i < fv.Len(); i++ {
						fv.Index(i).Set(vv.Index(i))
					}
				case reflect.Int:
					if vv.Kind() == reflect.String {
						if convint, err := strconv.Atoi(vv.String()); err == nil {
							covint64 := int64(convint)
							if fv.Index(0).OverflowInt(covint64) {
								return errors.New("value of field " + key + " overflows int")
							}
							fv.Set(reflect.MakeSlice(fv.Type(), 1, 1))
							fv.Index(0).SetInt(covint64)
							continue
						} else {
							return errors.New("cant convert value of field " + key + " to int, err: " + err.Error())
						}
					}
					fv.Set(reflect.MakeSlice(fv.Type(), vv.Len(), vv.Len()))
					for i := 0; i < fv.Len(); i++ {
						if convint, err := strconv.Atoi(vv.Index(i).String()); err == nil {
							covint64 := int64(convint)
							if fv.Index(0).OverflowInt(covint64) {
								return errors.New("value of field " + key + " overflows int")
							}
							fv.Index(i).SetInt(covint64)
						} else {
							return errors.New("cant convert value of field " + key + " to int, err: " + err.Error())
						}
					}
				default:
					return errors.New("unsupportable slice type of field " + key)
				}

			default:
				return errors.New("unsupportable type of field " + key)
			}
		}
	}
	return nil
}
