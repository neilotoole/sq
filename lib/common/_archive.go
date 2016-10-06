package common

//
//func NewRowReceiver(fields []driver.Field) (*RowReceiver, error) {
//
//	//scan :=
//
//	dest := make([]interface{}, len(fields))
//
//	for i, field := range fields {
//
//		//f := field.FieldType
//		switch field.FieldType {
//		case driver.FieldTypeNULL:
//			dest[i] = nil
//			continue
//
//		// Numeric Types
//		case driver.FieldTypeTiny:
//			// TODO: need to check for nullability here, and return sql.NullInt64{}
//			if field.Flags.IsSet(driver.FlagNotNULL) {
//				dest[i] = new(int64)
//
//			} else {
//				dest[i] = new(sql.NullInt64)
//			}
//			//sql.NullInt64{}
//			//dest[i] = new(int64)
//			//if rows.columns[i].flags&flagUnsigned != 0 {
//			//	dest[i] = int64(data[pos])
//			//} else {
//			//	dest[i] = int64(int8(data[pos]))
//			//}
//			//pos++
//			continue
//
//		case driver.FieldTypeShort, driver.FieldTypeYear:
//			if field.Flags.IsSet(driver.FlagNotNULL) {
//				dest[i] = new(int64)
//
//			} else {
//				dest[i] = new(sql.NullInt64)
//			}
//			//if rows.columns[i].flags&flagUnsigned != 0 {
//			//	dest[i] = int64(binary.LittleEndian.Uint16(data[pos : pos+2]))
//			//} else {
//			//	dest[i] = int64(int16(binary.LittleEndian.Uint16(data[pos : pos+2])))
//			//}
//			//pos += 2
//			continue
//
//		case driver.FieldTypeInt24, driver.FieldTypeLong:
//			if field.Flags.IsSet(driver.FlagNotNULL) {
//				dest[i] = new(int64)
//
//			} else {
//				dest[i] = new(sql.NullInt64)
//			}
//			//if rows.columns[i].flags&flagUnsigned != 0 {
//			//	dest[i] = int64(binary.LittleEndian.Uint32(data[pos : pos+4]))
//			//} else {
//			//	dest[i] = int64(int32(binary.LittleEndian.Uint32(data[pos : pos+4])))
//			//}
//			//pos += 4
//			continue
//
//		case driver.FieldTypeLongLong:
//			if field.Flags.IsSet(driver.FlagNotNULL) {
//				dest[i] = new(int64)
//
//			} else {
//				dest[i] = new(sql.NullInt64)
//			}
//
//			//if rows.columns[i].flags&flagUnsigned != 0 {
//			//	val := binary.LittleEndian.Uint64(data[pos : pos+8])
//			//	if val > math.MaxInt64 {
//			//		dest[i] = uint64ToString(val)
//			//	} else {
//			//		dest[i] = int64(val)
//			//	}
//			//} else {
//			//	dest[i] = int64(binary.LittleEndian.Uint64(data[pos : pos+8]))
//			//}
//			//pos += 8
//			continue
//
//		case driver.FieldTypeFloat:
//			if field.Flags.IsSet(driver.FlagNotNULL) {
//				dest[i] = new(float64)
//
//			} else {
//				dest[i] = new(sql.NullFloat64)
//			}
//			//dest[i] = new(float64)
//			//dest[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[pos : pos+4])))
//			//pos += 4
//			continue
//
//		case driver.FieldTypeDouble:
//			if field.Flags.IsSet(driver.FlagNotNULL) {
//				dest[i] = new(float64)
//
//			} else {
//				dest[i] = new(sql.NullFloat64)
//			}
//			//dest[i] = math.Float64frombits(binary.LittleEndian.Uint64(data[pos : pos+8]))
//			//pos += 8
//			continue
//
//		// Length coded Binary Strings
//		case driver.FieldTypeDecimal, driver.FieldTypeNewDecimal, driver.FieldTypeVarChar,
//			driver.FieldTypeBit, driver.FieldTypeEnum, driver.FieldTypeSet,
//			driver.FieldTypeVarString, driver.FieldTypeString, driver.FieldTypeGeometry, driver.FieldTypeJSON:
//
//			// TODO: For binary types, we should probably use []byte, as we will
//			// later want to convert to base64 when printing
//
//			if field.Flags.IsSet(driver.FlagNotNULL) {
//				dest[i] = new(string)
//			} else {
//				dest[i] = new(sql.NullString)
//			}
//			continue
//
//		// Length coded Binary Strings
//		case driver.FieldTypeTinyBLOB,
//			driver.FieldTypeMediumBLOB, driver.FieldTypeLongBLOB, driver.FieldTypeBLOB:
//
//			// TODO: For binary types, we should probably use []byte, as we will
//			// later want to convert to base64 when printing
//			if field.Flags.IsSet(driver.FlagBinary) {
//				dest[i] = &[]byte{}
//			} else {
//				if field.Flags.IsSet(driver.FlagNotNULL) {
//					dest[i] = new(string)
//				} else {
//					dest[i] = new(sql.NullString)
//				}
//			}
//
//			continue
//		//var isNull bool
//		//var n int
//		//dest[i], isNull, n, err = readLengthEncodedString(data[pos:])
//		//pos += n
//		//if err == nil {
//		//	if !isNull {
//		//		continue
//		//	} else {
//		//		dest[i] = nil
//		//		continue
//		//	}
//		//}
//		//return err
//
//		case
//			driver.FieldTypeDate, driver.FieldTypeNewDate, // Date YYYY-MM-DD
//			driver.FieldTypeTime,                                // Time [-][H]HH:MM:SS[.fractal]
//			driver.FieldTypeTimestamp, driver.FieldTypeDateTime: // Timestamp YYYY-MM-DD HH:MM:SS[.fractal]
//
//			//dest[i] = new(time.Time)
//			if field.Flags.IsSet(driver.FlagNotNULL) {
//				dest[i] = new(string)
//			} else {
//				dest[i] = new(sql.NullString)
//			}
//			continue
//		//num, isNull, n := readLengthEncodedInteger(data[pos:])
//		//pos += n
//		//
//		//switch {
//		//case isNull:
//		//	dest[i] = nil
//		//	continue
//		//case rows.columns[i].fieldType == fieldTypeTime:
//		//	// database/sql does not support an equivalent to TIME, return a string
//		//	var dstlen uint8
//		//	switch decimals := rows.columns[i].decimals; decimals {
//		//	case 0x00, 0x1f:
//		//		dstlen = 8
//		//	case 1, 2, 3, 4, 5, 6:
//		//		dstlen = 8 + 1 + decimals
//		//	default:
//		//		return fmt.Errorf(
//		//			"protocol error, illegal decimals value %d",
//		//			rows.columns[i].decimals,
//		//		)
//		//	}
//		//	dest[i], err = formatBinaryDateTime(data[pos:pos+int(num)], dstlen, true)
//		//case rows.mc.parseTime:
//		//	dest[i], err = parseBinaryDateTime(num, data[pos:], rows.mc.cfg.Loc)
//		//default:
//		//	var dstlen uint8
//		//	if rows.columns[i].fieldType == fieldTypeDate {
//		//		dstlen = 10
//		//	} else {
//		//		switch decimals := rows.columns[i].decimals; decimals {
//		//		case 0x00, 0x1f:
//		//			dstlen = 19
//		//		case 1, 2, 3, 4, 5, 6:
//		//			dstlen = 19 + 1 + decimals
//		//		default:
//		//			return fmt.Errorf(
//		//				"protocol error, illegal decimals value %d",
//		//				rows.columns[i].decimals,
//		//			)
//		//		}
//		//	}
//		//	dest[i], err = formatBinaryDateTime(data[pos:pos+int(num)], dstlen, false)
//		//}
//		//
//		//if err == nil {
//		//	pos += int(num)
//		//	continue
//		//} else {
//		//	return err
//		//}
//
//		// Please report if this happens!
//		default:
//			if field.Flags.IsSet(driver.FlagNotNULL) {
//				dest[i] = new(string)
//			} else {
//				dest[i] = new(sql.NullString)
//			}
//		//return nil, fmt.Errorf("unknown field type %s", field.FieldType)
//		}
//
//	}
//
//	//
//	//for i := range scans {
//	//
//	//}
//	r := &RowReceiver{Values: dest, Fields: fields}
//
//	return r, nil
//
//}
