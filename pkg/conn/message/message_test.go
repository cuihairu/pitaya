package message

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/pitaya/v3/pkg/helpers"
)

var update = flag.Bool("update", false, "update .golden files")

// mustGzipData is a repetitive payload that reliably shrinks under zlib on
// all supported Go versions, so the encoder sets the gzip mask bit. Smaller
// inputs (e.g. 39 bytes of "blabla") may be stored uncompressed on newer Go
// versions, which would silently defeat the gzip assertion.
var mustGzipData = []byte(strings.Repeat("blabla", 20))

func resetDicts(t *testing.T) {
	t.Helper()
	routesCodesMutex.Lock()
	defer routesCodesMutex.Unlock()
	routes = make(map[string]uint16)
	codes = make(map[uint16]string)
}

func TestNew(t *testing.T) {
	t.Parallel()

	message := New()

	assert.NotNil(t, message)
}

var encodeTables = map[string]struct {
	message *Message
	routes  map[string]uint16
	msgErr  bool
	gzip    uint8
	err     error
}{
	"test_wrong_type": {&Message{Type: 0xff, Data: []byte{}}, nil, false, 0x0, ErrWrongMessageType},

	"test_request_type": {&Message{Type: Request, Route: "a", Data: []uint8{}}, nil, false, 0x0, nil},
	"test_request_type_compressed": {&Message{Type: Request, Route: "a", Data: []byte{}, compressed: true},
		map[string]uint16{"a": 1}, false, 0x0, nil},

	"test_notify_type": {&Message{Type: Notify, Route: "a", Data: []byte{}}, nil, false, 0x0, nil},
	"test_notify_type_compressed": {&Message{Type: Notify, Route: "a", Data: []byte{}, compressed: true},
		map[string]uint16{"a": 1}, false, 0x0, nil},

	"test_push_type": {&Message{Type: Push, Route: "a", Data: []byte{}}, nil, false, 0x0, nil},
	"test_push_type_compressed": {&Message{Type: Push, Route: "a", Data: []byte{}, compressed: true},
		map[string]uint16{"a": 1}, false, 0x0, nil},

	"test_reponse_type":           {&Message{Type: Response, Data: []byte{}}, nil, false, 0x0, nil},
	"test_reponse_type_with_data": {&Message{Type: Response, Data: []byte{0x01}}, nil, false, 0x0, nil},
	"test_reponse_type_with_id":   {&Message{Type: Response, ID: 129, Data: []byte{}}, nil, false, 0x0, nil},

	"test_reponse_type_with_error": {&Message{Type: Response, Data: []byte{0x01}, Err: true}, nil, true, 0x0, nil},
	"test_must_gzip": {&Message{Type: Response,
		Data: mustGzipData, Err: true}, nil, true, 0x10, nil},
}

func TestEncode(t *testing.T) {
	for name, table := range encodeTables {
		t.Run(name, func(t *testing.T) {
			message := table.message
			originalData := append([]byte(nil), message.Data...)
			SetDictionary(table.routes)

			messageEncoder := NewMessagesEncoder(table.gzip == 0x10)
			result, err := messageEncoder.Encode(message)

			if err == nil {
				if table.gzip == 0x10 {
					// zlib output bytes vary across Go versions, so the
					// compressed payload must be verified by round-trip
					// (decode back to the original data) rather than exact
					// golden bytes.
					assert.Equal(t, byte(0x10), result[0]&gzipMask)
					decoded, derr := Decode(result)
					assert.NoError(t, derr)
					assert.Equal(t, originalData, decoded.Data)
				} else {
					gp := filepath.Join("fixtures", name+".golden")
					if *update {
						t.Log("updating golden file")
						helpers.WriteFile(t, gp, result)
					}
					expected := helpers.ReadFile(t, gp)
					assert.Equal(t, expected, result)
				}
			} else {
				assert.Nil(t, result)
			}

			assert.Equal(t, table.err, err)

			resetDicts(t)
		})
	}
}

var decodeTables = map[string]struct {
	message *Message
	routes  map[string]uint16
	msgErr  bool
	gzip    uint8
	err     error
}{
	"test_wrong_type": {&Message{Type: 0xff, Data: []byte{}}, nil, false, 0x0, ErrWrongMessageType},

	"test_request_type": {&Message{Type: Request, Route: "a", Data: []uint8{}}, nil, false, 0x0, nil},
	"test_request_type_compressed": {&Message{Type: Request, Route: "a", Data: []byte{}, compressed: true},
		map[string]uint16{"a": 1}, false, 0x0, nil},

	"test_notify_type": {&Message{Type: Notify, Route: "a", Data: []byte{}}, nil, false, 0x0, nil},
	"test_notify_type_compressed": {&Message{Type: Notify, Route: "a", Data: []byte{}, compressed: true},
		map[string]uint16{"a": 1}, false, 0x0, nil},

	"test_push_type": {&Message{Type: Push, Route: "a", Data: []byte{}}, nil, false, 0x0, nil},
	"test_push_type_compressed": {&Message{Type: Push, Route: "a", Data: []byte{}, compressed: true},
		map[string]uint16{"a": 1}, false, 0x0, nil},

	"test_reponse_type":           {&Message{Type: Response, Data: []byte{}}, nil, false, 0x0, nil},
	"test_reponse_type_with_data": {&Message{Type: Response, Data: []byte{0x01}}, nil, false, 0x0, nil},
	"test_reponse_type_with_id":   {&Message{Type: Response, ID: 129, Data: []byte{}}, nil, false, 0x0, nil},

	"test_reponse_type_with_error": {&Message{Type: Response, Data: []byte{0x01}, Err: true}, nil, true, 0x0, nil},
	"test_must_gzip": {&Message{Type: Response,
		Data: mustGzipData, Err: true}, nil, true, 0x10, nil},
}

func TestDecode(t *testing.T) {
	for name, table := range decodeTables {
		t.Run(name, func(t *testing.T) {
			SetDictionary(table.routes)

			gp := filepath.Join("fixtures", name+".golden")
			encoded := helpers.ReadFile(t, gp)

			message, err := Decode(encoded)

			if err == nil {
				assert.Equal(t, table.message, message)
			}
			if name == "test_wrong_type" {
				assert.EqualError(t, ErrInvalidMessage, err.Error())
			} else {
				assert.Equal(t, table.err, err)
			}
			resetDicts(t)
		})
	}
}

var dictTables = map[string]struct {
	dicts  []map[string]uint16
	routes map[string]uint16
	codes  map[uint16]string
	err    error
}{
	"test_add_new_route": {[]map[string]uint16{{"a": 1}}, map[string]uint16{"a": 1},
		map[uint16]string{1: "a"}, nil},
	"test_add_new_routes": {[]map[string]uint16{{"a": 1}, {"b": 2}}, map[string]uint16{"a": 1, "b": 2},
		map[uint16]string{1: "a", 2: "b"}, nil},
	"test_override_route": {[]map[string]uint16{{"a": 1}, {"a": 2}}, map[string]uint16{"a": 1},
		map[uint16]string{1: "a"}, errors.New("duplicated route(route: a, code: 1)")},
	"test_override_code": {[]map[string]uint16{{"a": 1}, {"b": 1}}, map[string]uint16{"a": 1},
		map[uint16]string{1: "a"}, errors.New("duplicated route(route: b, code: 1)")},
}

func TestSetDictionary(t *testing.T) {
	for name, table := range dictTables {
		t.Run(name, func(t *testing.T) {
			for _, dict := range table.dicts {
				SetDictionary(dict)
			}

			assert.Equal(t, table.routes, routes)
			assert.Equal(t, table.codes, codes)

			resetDicts(t)
		})
	}
}

func TestSetDictionaryRace(t *testing.T) {
	defer resetDicts(t)

	done := make(chan bool, 2)

	setDictRace := func(dict map[string]uint16) {
		assert.Nil(t, SetDictionary(dict))
		done <- true
	}

	go setDictRace(map[string]uint16{"a": 1})
	go setDictRace(map[string]uint16{"b": 2})

	// wait for both setDictRace to finish
	<-done
	<-done

	expected_codes := map[uint16]string{1: "a", 2: "b"}
	assert.EqualValues(t, expected_codes, codes)

	expected_routes := map[string]uint16{"a": 1, "b": 2}
	assert.EqualValues(t, expected_routes, routes)
}

func TestGetDictionary(t *testing.T) {
	defer resetDicts(t)
	expected := map[string]uint16{"a": 1, "b": 2}
	assert.Nil(t, SetDictionary(expected))

	dict := GetDictionary()
	assert.Equal(t, expected, dict)

	// make sure we're copying the routes maps
	assert.NotEqual(t, fmt.Sprintf("%p", routes), fmt.Sprintf("%p", dict))
}

func TestDecodeEmptyRouteLengthDoesNotPanic(t *testing.T) {
	// A Request carrying an ID but no route-length byte must yield
	// ErrInvalidMessage instead of panicking on data[offset] (offset == size).
	_, err := Decode([]byte{0x00, 0x01})
	assert.EqualError(t, err, ErrInvalidMessage.Error())
}

func TestDecodeRejectsOverlongMessageIDVarint(t *testing.T) {
	// Request (type 0) with an ID whose varint has a 9th continuation byte:
	// the ID would exceed 64 bits and the decoder must reject it rather than
	// treat the route-length/data bytes as part of the ID.
	data := []byte{0x00, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80}
	_, err := Decode(data)
	assert.EqualError(t, err, ErrInvalidMessage.Error())
}

func TestDecodeRejectsUnterminatedMessageIDVarint(t *testing.T) {
	// Request (type 0) whose ID varint never terminates before the payload
	// ends; the decoder must not swallow the whole payload silently.
	data := []byte{0x00, 0x80, 0x80}
	_, err := Decode(data)
	assert.EqualError(t, err, ErrInvalidMessage.Error())
}

func TestEncodeDecodeFull64BitID(t *testing.T) {
	enc := NewMessagesEncoder(false)
	for _, id := range []uint{1 << 56, 1 << 60, 1<<63 - 1, 1 << 63} {
		msg := &Message{Type: Request, ID: id, Route: "a", Data: []byte{}}
		data, err := enc.Encode(msg)
		assert.NoError(t, err)
		decoded, err := Decode(data)
		assert.NoError(t, err)
		assert.Equal(t, id, decoded.ID)
	}
}

func TestDecodeRejectsOverflowingMessageIDVarint(t *testing.T) {
	// A 10th varint byte carrying more than bit 63 would overflow the 64-bit
	// id; the decoder must reject it rather than wrap the id around.
	data := []byte{0x00, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x02, 0x00}
	_, err := Decode(data)
	assert.EqualError(t, err, ErrInvalidMessage.Error())
}
