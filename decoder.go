package confdecoder

import (
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type ParsedFileData struct {
	parseddata        filedata
	Keys              []string // пишутся, даже если для ключей нет значения
	NestedStructsMode byte
}

// (Дефолтный режим) Значения полям вложенных структур присваиваются так же, как и обычным полям
const NestedStructsModeOne byte = 1

// Значения всем полям вложенной структуры присваиваются одновременно по имени поля-структуры, т.е. все значения в одну строку в файле, пропуск значения считается ошибкой
const NestedStructsModeTwo byte = 2

type filedata map[string]interface{}

func ParseFile(filepath string) (*ParsedFileData, error) {
	rawdata, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	pfd := &ParsedFileData{parseddata: make(filedata), NestedStructsMode: 1}

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
		_, reassignation := pfd.parseddata[elems[0]]
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
		if !reassignation {
			pfd.Keys = append(pfd.Keys, elems[0])
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
			if !fv.CanSet() {
				continue
			}
		switching:
			switch fv.Kind() {
			case reflect.Ptr:
				if fv.IsNil() {
					fv.Set(reflect.New(fv.Type().Elem()))
				}
				fv = fv.Elem()
				goto switching
			case reflect.Struct:
				if fv.NumField() != 0 {
					if pfd.NestedStructsMode == NestedStructsModeOne {
						structsvalues = append(structsvalues, fv)
						continue
					}
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
		switch fv.Kind() {
		case reflect.Struct:
			if fv.NumField() == 0 {
				return nil
			}
			if vv.Kind() != reflect.Slice || fv.NumField() != vv.Len() {
				return errors.New("mismatch of num of values in file with num of fields in given struct " + fieldname)
			}
			l := vv.Len()
			var ffv reflect.Value
			for i := 0; i < l; i++ {
				ffv = fv.Field(i)
				if ffv.Type().Kind() == reflect.Ptr {
					if fv.IsNil() {
						ffv.Set(reflect.New(ffv.Type().Elem()))
					}
					ffv = ffv.Elem()
				}
				if !ffv.IsValid() || !ffv.CanSet() {
					continue
				}
				switch ffv.Kind() {
				case reflect.String:
					if vv.Index(i).Kind() == reflect.String {
						ffv.SetString(vv.Index(i).String())
					} else {
						return errors.New("(1)cant set value of type " + vv.Index(i).Kind().String() + " to field of struct named " + fieldname)
					}
				case reflect.Int:
					if vv.Index(i).Kind() == reflect.String {
						if convint, err := strconv.Atoi(vv.Index(i).String()); err == nil {
							covint64 := int64(convint)
							if ffv.OverflowInt(covint64) {
								return errors.New("value " + vv.Index(i).String() + " of field " + fieldname + " overflows int")
							}
							ffv.SetInt(covint64)
						} else {
							return errors.New("cant convert value " + vv.Index(i).String() + " of field " + fieldname + " to int, err: " + err.Error())
						}
					} else {
						return errors.New("(2)cant set value of type " + vv.Index(i).Kind().String() + " to field of struct named " + fieldname)
					}
				case reflect.Slice:
					switch ffv.Type().Elem().Kind() {
					case reflect.String:
						if vv.Index(i).Kind() == reflect.String {
							elems := strings.Split(strings.Trim(vv.Index(i).String(), "{}[]"), ",")
							ffv.Set(reflect.MakeSlice(ffv.Type(), len(elems), len(elems)))
							for k := 0; k < len(elems); k++ {
								ffv.Index(k).SetString(elems[k])
							}
						} else {
							return errors.New("(3)cant set value of type " + vv.Index(i).Kind().String() + " as elem of slice to field of struct named " + fieldname)
						}
					case reflect.Int:
						if vv.Index(i).Kind() == reflect.String {
							elems := strings.Split(strings.Trim(vv.Index(i).String(), "{}[]"), ",")
							ffv.Set(reflect.MakeSlice(ffv.Type(), len(elems), len(elems)))
							for k := 0; k < len(elems); k++ {
								if convint, err := strconv.Atoi(elems[k]); err == nil {
									covint64 := int64(convint)
									if ffv.Index(k).OverflowInt(covint64) {
										return errors.New("value " + elems[k] + " of field " + fieldname + " overflows int")
									}
									ffv.Index(k).SetInt(covint64)
								} else {
									return errors.New("cant convert value of field " + fieldname + " to int, err: " + err.Error())
								}
							}
						} else {
							return errors.New("(4)cant set value of type " + vv.Index(i).Kind().String() + " as elem of slice to field of struct named " + fieldname)
						}
					default:
						return errors.New("unsupportable slice " + ffv.Type().Elem().Kind().String() + " type of field \"" + fieldname + "\"")
					}
				default:
					return errors.New("unsupportable type " + ffv.Kind().String() + " for a nested struct's field")
				}
			}

		case reflect.String:
			if vv.Kind() == reflect.String {
				fv.SetString(vv.String())
			} else if vv.Kind() == reflect.Slice && vv.Type().Elem().Kind() == reflect.String {
				sl := make([]string, vv.Len())
				for i := 0; i < len(sl); i++ {
					sl[i] = vv.Index(i).String()
				}
				fv.SetString(strings.Join(sl, " "))
			} else {
				return errors.New("(5)cant set value of type " + vv.Kind().String() + " to field named " + fieldname)
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
				return errors.New("(6)cant set value of type slice to field named " + fieldname)
			}

		case reflect.Slice:
			switch fv.Type().Elem().Kind() {
			case reflect.String:
				// if vv.Kind() == reflect.String {
				// 	fv.Set(reflect.MakeSlice(fv.Type(), 1, 1))
				// 	fv.Index(0).Set(vv)
				// 	return nil
				// }
				// fv.Set(reflect.MakeSlice(fv.Type(), vv.Len(), vv.Len()))
				// for i := 0; i < fv.Len(); i++ {
				// 	fv.Index(i).Set(vv.Index(i))
				// }
				var vvstr string
				if vv.Kind() == reflect.String {
					vvstr = vv.String()
				} else if vv.Kind() == reflect.Slice && vv.Type().Elem().Kind() == reflect.String {
					vvstr = strings.Join(vv.Interface().([]string), " ")
				} else {
					return errors.New("(7)cant set value of type " + vv.Kind().String() + " as elem of slice to field of struct named " + fieldname)
				}

				elems := strings.Split(strings.Trim(vvstr, "{}[]"), ",")
				fv.Set(reflect.MakeSlice(fv.Type(), len(elems), len(elems)))
				for k := 0; k < len(elems); k++ {
					fv.Index(k).SetString(elems[k])
				}

			case reflect.Int:
				// if vv.Kind() == reflect.String {
				// 	if convint, err := strconv.Atoi(vv.String()); err == nil {
				// 		covint64 := int64(convint)
				// 		if fv.Index(0).OverflowInt(covint64) {
				// 			return errors.New("value of field " + fieldname + " overflows int")
				// 		}
				// 		fv.Set(reflect.MakeSlice(fv.Type(), 1, 1))
				// 		fv.Index(0).SetInt(covint64)
				// 		return nil
				// 	} else {
				// 		return errors.New("cant convert value of field " + fieldname + " to int, err: " + err.Error())
				// 	}
				// }
				// fv.Set(reflect.MakeSlice(fv.Type(), vv.Len(), vv.Len()))
				// for i := 0; i < fv.Len(); i++ {
				// 	if convint, err := strconv.Atoi(vv.Index(i).String()); err == nil {
				// 		covint64 := int64(convint)
				// 		if fv.Index(0).OverflowInt(covint64) {
				// 			return errors.New("value of field " + fieldname + " overflows int")
				// 		}
				// 		fv.Index(i).SetInt(covint64)
				// 	} else {
				// 		return errors.New("cant convert value of field " + fieldname + " to int, err: " + err.Error())
				// 	}
				// }

				if vv.Kind() == reflect.String {
					elems := strings.Split(strings.Trim(vv.String(), "{}[]"), ",")
					fv.Set(reflect.MakeSlice(fv.Type(), len(elems), len(elems)))
					for k := 0; k < len(elems); k++ {
						if convint, err := strconv.Atoi(elems[k]); err == nil {
							covint64 := int64(convint)
							if fv.Index(k).OverflowInt(covint64) {
								return errors.New("value " + elems[k] + " of field " + fieldname + " overflows int")
							}
							fv.Index(k).SetInt(covint64)
						} else {
							return errors.New("cant convert value of field " + fieldname + " to int, err: " + err.Error())
						}
					}
				} else {
					return errors.New("(8)cant set value of type " + vv.Kind().String() + " as elem of slice to field named " + fieldname)
				}

			default:
				return errors.New("unsupportable slice " + fv.Type().Elem().Kind().String() + " type of field \"" + fieldname + "\"")
			}

		default:
			return errors.New("unsupportable type " + fv.Kind().String() + " of field \"" + fieldname + "\"")
		}
	}
	return nil
}
