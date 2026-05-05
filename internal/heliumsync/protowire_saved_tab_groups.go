package heliumsync

import (
	"encoding/hex"
	"errors"

	"google.golang.org/protobuf/encoding/protowire"
)

type stgEntity struct {
	Kind, GUID, Title, URL, GroupGUID string
	CreationTime, UpdateTime          int64
	Version                           int32
	Color, Position                   int64
}

func decodeLevelDBEntity(valHex string) (stgEntity, error) {
	val, err := hex.DecodeString(valHex)
	if err != nil {
		return stgEntity{}, err
	}
	spec, err := decodeWrapperSpecifics(val)
	if err != nil {
		return stgEntity{}, err
	}
	return decodeSpecifics(spec)
}

func decodeWrapperSpecifics(b []byte) ([]byte, error) {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		b = b[n:]
		if num == 2 && typ == protowire.BytesType {
			v, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return nil, protowire.ParseError(m)
			}
			return v, nil
		}
		m := protowire.ConsumeFieldValue(num, typ, b)
		if m < 0 {
			return nil, protowire.ParseError(m)
		}
		b = b[m:]
	}
	return nil, errors.New("wrapper missing specifics")
}

func decodeSpecifics(b []byte) (stgEntity, error) {
	var e stgEntity
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return e, protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			v, m := protowire.ConsumeString(b)
			e.GUID, b = v, b[m:]
		case 2:
			v, m := protowire.ConsumeVarint(b)
			e.CreationTime, b = int64(v), b[m:]
		case 3:
			v, m := protowire.ConsumeVarint(b)
			e.UpdateTime, b = int64(v), b[m:]
		case 4:
			v, m := protowire.ConsumeBytes(b)
			decodeGroup(v, &e)
			b = b[m:]
		case 5:
			v, m := protowire.ConsumeBytes(b)
			decodeTab(v, &e)
			b = b[m:]
		case 7:
			v, m := protowire.ConsumeVarint(b)
			e.Version, b = int32(v), b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, typ, b)
			if m < 0 {
				return e, protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return e, nil
}

func decodeGroup(b []byte, e *stgEntity) {
	e.Kind = "group"
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return
		}
		b = b[n:]
		switch num {
		case 1:
			v, m := protowire.ConsumeVarint(b)
			e.Position, b = int64(v), b[m:]
		case 2:
			v, m := protowire.ConsumeString(b)
			e.Title, b = v, b[m:]
		case 3:
			v, m := protowire.ConsumeVarint(b)
			e.Color, b = int64(v), b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, typ, b)
			if m < 0 {
				return
			}
			b = b[m:]
		}
	}
}

func decodeTab(b []byte, e *stgEntity) {
	e.Kind = "tab"
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return
		}
		b = b[n:]
		switch num {
		case 1:
			v, m := protowire.ConsumeString(b)
			e.GroupGUID, b = v, b[m:]
		case 2:
			v, m := protowire.ConsumeVarint(b)
			e.Position, b = int64(v), b[m:]
		case 3:
			v, m := protowire.ConsumeString(b)
			e.URL, b = v, b[m:]
		case 4:
			v, m := protowire.ConsumeString(b)
			e.Title, b = v, b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, typ, b)
			if m < 0 {
				return
			}
			b = b[m:]
		}
	}
}

func encodeWrapperSpecifics(spec []byte) []byte {
	var out []byte
	out = protowire.AppendTag(out, 1, protowire.VarintType)
	out = protowire.AppendVarint(out, 1)
	out = protowire.AppendTag(out, 2, protowire.BytesType)
	out = protowire.AppendBytes(out, spec)
	return out
}

func encodeGroup(g map[string]any) []byte {
	var group []byte
	group = appendVarint(group, 1, uint64(int64Value(g["position"])))
	group = appendString(group, 2, str(g["title"]))
	group = appendVarint(group, 3, uint64(int64Value(g["color"])))

	var spec []byte
	spec = appendString(spec, 1, str(g["guid"]))
	spec = appendVarint(spec, 2, uint64(int64Value(g["creation_time"])))
	spec = appendVarint(spec, 3, uint64(int64Value(g["update_time"])))
	spec = appendBytes(spec, 4, group)
	if _, ok := g["version"]; ok {
		spec = appendVarint(spec, 7, uint64(int64Value(g["version"])))
	}
	return spec
}

func encodeTab(t map[string]any) []byte {
	var tab []byte
	tab = appendString(tab, 1, str(t["group_guid"]))
	tab = appendVarint(tab, 2, uint64(int64Value(t["position"])))
	tab = appendString(tab, 3, str(t["url"]))
	tab = appendString(tab, 4, str(t["title"]))

	var spec []byte
	spec = appendString(spec, 1, str(t["guid"]))
	spec = appendVarint(spec, 2, uint64(int64Value(t["creation_time"])))
	spec = appendVarint(spec, 3, uint64(int64Value(t["update_time"])))
	spec = appendBytes(spec, 5, tab)
	if _, ok := t["version"]; ok {
		spec = appendVarint(spec, 7, uint64(int64Value(t["version"])))
	}
	return spec
}

func appendString(b []byte, n protowire.Number, s string) []byte {
	b = protowire.AppendTag(b, n, protowire.BytesType)
	return protowire.AppendString(b, s)
}

func appendBytes(b []byte, n protowire.Number, v []byte) []byte {
	b = protowire.AppendTag(b, n, protowire.BytesType)
	return protowire.AppendBytes(b, v)
}

func appendVarint(b []byte, n protowire.Number, v uint64) []byte {
	b = protowire.AppendTag(b, n, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}
