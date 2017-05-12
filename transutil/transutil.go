// Package transutil provides a set of example utilites for converting between
// common data formats using an io.Reader. Currently supporte are JSON,
// MsgPack, and ProtoBuf. Also provided is Gzipper/Gunzipper readers.
package transutil

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"

	msgpack "gopkg.in/vmihailenco/msgpack.v2"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/tidwall/transform"
)

// JSONToPrettyJSON returns an io.Reader that converts JSON messages
// by making them more human readable using indentation and linebreaks.
func JSONToPrettyJSON(r io.Reader) *transform.Transformer {
	dec := json.NewDecoder(r)
	return transform.NewTransformer(func() ([]byte, error) {
		var v interface{}
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		return json.MarshalIndent(&v, "", "  ")
	})
}

// JSONToUglyJSON returns an io.Reader that converts JSON messages
// by removing all unneeded whitespace.
func JSONToUglyJSON(r io.Reader) *transform.Transformer {
	dec := json.NewDecoder(r)
	return transform.NewTransformer(func() ([]byte, error) {
		var v interface{}
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		return json.Marshal(&v)
	})
}

// JSONToProtoBuf returns an io.Reader that converts JSON messages
// into Protocol Buffers.
//
// The pb param is the proto buffer definition
// that conforms to the proto.Message interface. This param is only
// used during the conversion process and MUST NOT be used after
// calling this function.
//
// The multimessage param is used for sending multiple messages over the same
// stream. When this param is set, additional varint bytes are added to
// the beginning of each message. Otherwise only one message is allowed.
func JSONToProtoBuf(r io.Reader, pb proto.Message, multimessage bool) *transform.Transformer {
	var count int
	var dec = json.NewDecoder(r)
	return transform.NewTransformer(func() ([]byte, error) {
		if err := jsonpb.UnmarshalNext(dec, pb); err != nil {
			return nil, err
		}
		if count > 0 && !multimessage {
			return nil, errors.New("not a multimessage stream")
		}
		data, err := proto.Marshal(pb)
		if err != nil {
			return nil, err
		}
		if multimessage {
			data = append(proto.EncodeVarint(uint64(len(data))), data...)
		}
		count++
		return data, err
	})
}

// ProtoBufToJSON returns an io.Reader that converts Proto Buffer
// messages into JSON.
//
// The pb param is the proto buffer definition
// that conforms to the proto.Message interface. This param is only
// used during the conversion process and MUST NOT be used after
// calling this function.
//
// The multimessage param is used for sending multiple messages over the same
// stream. When this param is set, additional varint bytes are added to
// the beginning of each message. Otherwise only one message is allowed.
func ProtoBufToJSON(r io.Reader, pb proto.Message, multimessage bool) *transform.Transformer {
	if !multimessage {
		return transform.NewTransformer(func() ([]byte, error) {
			if data, err := ioutil.ReadAll(r); err != nil {
				return nil, err
			} else if len(data) == 0 {
				return nil, io.EOF
			} else if err = proto.Unmarshal(data, pb); err != nil {
				return nil, err
			}
			// transform the pb to json. let's use the default options that
			// golang/protobuf recommends.
			str, err := (&jsonpb.Marshaler{}).MarshalToString(pb)
			return []byte(str), err
		})
	}
	var szb []byte // reused
	var msg []byte // reused
	var br = bufio.NewReader(r)
	return transform.NewTransformer(func() ([]byte, error) {
		var err error
		// read the size
		var sz uint64
		szb = szb[:0]
		for {
			szb = append(szb, 0)
			szb[len(szb)-1], err = br.ReadByte()
			if err != nil {
				if err == io.EOF && len(szb) > 1 {
					// we have a partial varint, this is quite unexpected.
					return nil, io.ErrUnexpectedEOF
				}
				return nil, err
			}
			if szb[len(szb)-1]>>7 == 0 {
				// the most signifigant bit is zero. we now know the size.
				sz, _ = proto.DecodeVarint(szb)
				break
			}
		}
		// grow the message buffer if needed.
		mcap := uint64(len(msg))
		if sz >= mcap {
			if mcap == 0 {
				mcap = 1
			}
			for sz >= mcap {
				mcap *= 2
			}
			msg = make([]byte, mcap)
		}
		// read the message
		if _, err := io.ReadFull(br, msg[:sz]); err != nil {
			return nil, err
		}
		// unmarshal the message
		if err := proto.Unmarshal(msg[:sz], pb); err != nil {
			return nil, err
		}
		// transform the pb to json. let's use the default options that
		// golang/protobuf recommends.
		str, err := (&jsonpb.Marshaler{}).MarshalToString(pb)
		return []byte(str), err
	})
}

// MsgPackToJSON returns an io.Reader that converts MsgPack messages
// into JSON messages.
func MsgPackToJSON(r io.Reader) *transform.Transformer {
	dec := msgpack.NewDecoder(r)
	return transform.NewTransformer(func() ([]byte, error) {
		var v interface{}
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		// It's important that maps have the `map[string]interface{}`
		// signature, but for some reason the MsgPack decoder returns
		// `map[interface{}]interface{}`.
		// No sweat though, we'll just do a little recursive translation.
		v = remapKeysToStrings(v)
		return json.Marshal(&v)
	})
}

func remapKeysToStrings(v interface{}) interface{} {
	// let's check if the map has an interface{} key.
	if iv, ok := v.(map[interface{}]interface{}); ok {
		// create a new map with a string key
		nv := make(map[string]interface{})
		for k, v := range iv {
			// translate nested values
			nv[k.(string)] = remapKeysToStrings(v)
		}
		return nv
	}
	return v
}

// JSONToMsgPack returns an io.Reader that converts JSON messages
// into MsgPack messages.
func JSONToMsgPack(r io.Reader) *transform.Transformer {
	dec := json.NewDecoder(r)
	return transform.NewTransformer(func() ([]byte, error) {
		var v interface{}
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		return msgpack.Marshal(&v)
	})
}

// Gzipper will gzip the input reader
func Gzipper(r io.Reader) *transform.Transformer {
	var b bytes.Buffer
	var w = gzip.NewWriter(&b)
	var rbuf = make([]byte, 4096)
	return transform.NewTransformer(func() ([]byte, error) {
		for {
			b.Reset()
			n, err := r.Read(rbuf)
			if err != nil {
				w.Flush()
				w.Close()
				if len(b.Bytes()) > 0 {
					return b.Bytes(), nil
				}
				return nil, err
			}
			w.Write(rbuf[:n])
			if len(b.Bytes()) != 0 {
				break
			}
		}
		return b.Bytes(), nil
	})
}

// Gunzipper will gunzip the input reader
func Gunzipper(r io.Reader) *transform.Transformer {
	var zr *gzip.Reader
	var rbuf = make([]byte, 4096)
	return transform.NewTransformer(func() ([]byte, error) {
		var err error
		if zr == nil {
			zr, err = gzip.NewReader(r)
			if err != nil {
				return nil, err
			}
		}
		n, err := zr.Read(rbuf)
		if err != nil {
			zr.Close()
			return nil, err
		}
		return rbuf[:n], nil
	})
}
