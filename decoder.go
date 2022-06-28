package confdecoder

import (
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type ParsedFileData struct {
	parseddata filedata
	Keys       []string // пишутся, даже если для ключей нет значения
}

type filedata map[string]interface{}

func ParseFile(filepath string) (*ParsedFileData, error) {
	rawdata, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	pfd := &ParsedFileData{parseddata: make(filedata)}

	lines := strings.Split(string(rawdata), "\n")
	pfd.Keys = make([]string, 0, len(lines))

	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		elems := strings.Split(strings.TrimSpace(line), " ")
		if len(elems) == 1 {
			if len(elems[0]) != 0 {
				pfd.Keys = append(pfd.Keys, elems[0])
			}
			continue
		}
		if _, ok := pfd.parseddata[elems[0]]; !ok {
			if len(elems) > 2 {
				valueList := make([]string, 0, len(elems)-1)
				for i := 1; i < len(elems); i++ {
					if len(elems[i]) != 0 {
						valueList = append(valueList, elems[i])
					}
				}
				pfd.parseddata[elems[0]] = valueList
			} else {
				pfd.parseddata[elems[0]] = elems[1]
			}
			pfd.Keys = append(pfd.Keys, elems[0])
		} else {
			return nil, errors.New("two lines with the same name")
		}
	}

	if len(pfd.parseddata) == 0 {
		pfd.parseddata = nil // no err on empty file
	}
	return pfd, nil
}

func (pfd *ParsedFileData) DecodeTo(v ...interface{}) error {
	structsvalues := make([]reflect.Value, 0, len(v))

	for _, s := range v {
		pv := reflect.ValueOf(s)
		if pv.Kind() != reflect.Ptr {
			return errors.New("arg must be a pointer")
		}
		if pv.IsNil() {
			return errors.New("arg is nil")
		}
		sv := pv.Elem()
		if sv.Kind() != reflect.Struct {
			return errors.New("args must be a non-nil pointers to a structs")
		}
		if sv.NumField() != 0 {
			structsvalues = append(structsvalues, sv)
		}
	}

	for i := 0; i < len(structsvalues); i++ {
		l := structsvalues[i].NumField()

		for k := 0; k < l; k++ {

			fv := structsvalues[i].Field(k)
			fname := structsvalues[i].Type().Field(k).Name

		switching:
			switch fv.Kind() {
			case reflect.Ptr:
				if !fv.IsNil() {
					fv = fv.Elem()
					goto switching
				} else {
					continue
				}
			case reflect.Struct:
				if fv.NumField() != 0 {
					structsvalues = append(structsvalues, fv)
					continue
				}
			case reflect.Interface:
				fv = fv.Elem()
				goto switching
			}
			if fv.IsValid() && fv.CanSet() {
				if err := pfd.parseddata.decodeToField(fname, fv); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func DecodeFile(filepath string, v ...interface{}) error {
	pfd, err := ParseFile(filepath)
	if err != nil {
		return err
	}
	return pfd.DecodeTo(v...)
}
func (data filedata) decodeToField(fieldname string, fv reflect.Value) error {
	if datv := data[fieldname]; datv != nil {
		vv := reflect.ValueOf(datv)

		if fv.IsValid() && fv.CanSet() {
			switch fv.Kind() {
			case reflect.String:
				if vv.Kind() == reflect.String {
					fv.SetString(vv.String())
				} else {
					return errors.New("cant set value of type " + vv.Kind().String() + " to field named " + fieldname)
				}
			case reflect.Int:
				if vv.Kind() == reflect.String {
					if convint, err := strconv.Atoi(vv.String()); err == nil {
						covint64 := int64(convint)
						if fv.OverflowInt(covint64) {
							return errors.New("value of field " + fieldname + " overflows int")
						}
						fv.SetInt(covint64)
					} else {
						return errors.New("cant convert value of field " + fieldname + " to int, err: " + err.Error())
					}
				} else {
					return errors.New("cant set value of type slice to field named " + fieldname)
				}
			case reflect.Slice:
				switch fv.Type().Elem().Kind() {
				case reflect.String:
					if vv.Kind() == reflect.String {
						fv.Set(reflect.MakeSlice(fv.Type(), 1, 1))
						fv.Index(0).Set(vv)
						return nil
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
								return errors.New("value of field " + fieldname + " overflows int")
							}
							fv.Set(reflect.MakeSlice(fv.Type(), 1, 1))
							fv.Index(0).SetInt(covint64)
							return nil
						} else {
							return errors.New("cant convert value of field " + fieldname + " to int, err: " + err.Error())
						}
					}
					fv.Set(reflect.MakeSlice(fv.Type(), vv.Len(), vv.Len()))
					for i := 0; i < fv.Len(); i++ {
						if convint, err := strconv.Atoi(vv.Index(i).String()); err == nil {
							covint64 := int64(convint)
							if fv.Index(0).OverflowInt(covint64) {
								return errors.New("value of field " + fieldname + " overflows int")
							}
							fv.Index(i).SetInt(covint64)
						} else {
							return errors.New("cant convert value of field " + fieldname + " to int, err: " + err.Error())
						}
					}
				default:
					return errors.New("unsupportable slice " + fv.Type().Elem().Kind().String() + " type of field \"" + fieldname + "\"")
				}

			default:
				return errors.New("unsupportable type " + fv.Kind().String() + " of field \"" + fieldname + "\"")
			}
		}
	}

	return nil
}
