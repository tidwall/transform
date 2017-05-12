package transutil_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/tidwall/transform/transutil"
	"github.com/tidwall/transform/transutil/pbtest"
)

func TestGzip(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	data := make([]byte, 100000)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	zipper := transutil.Gzipper(bytes.NewBuffer(data))
	zipped, err := ioutil.ReadAll(zipper)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(data, zipped) {
		t.Fatal("matched")
	}
	b, err := zipper.ReadMessage()
	if err != io.EOF {
		t.Fatal(err)
	}
	if len(b) != 0 {
		t.Fatal("not zero")
	}
	unzipper := transutil.Gunzipper(bytes.NewBuffer(zipped))
	unzipped, err := ioutil.ReadAll(unzipper)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, unzipped) {
		t.Fatal("not matched")
	}
	b, err = unzipper.ReadMessage()
	if err != io.EOF {
		t.Fatal(err)
	}
	if len(b) != 0 {
		t.Fatal("not zero")
	}
}

func cleanJSON(a string) string {
	var s string
	var v interface{}
	dec := json.NewDecoder(bytes.NewBufferString(a))
	for {
		if err := dec.Decode(&v); err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		s += string(b)
	}
	return s
}

func matchingJSON(a, b string) bool {
	return cleanJSON(a) == cleanJSON(b)
}

func TestJSONToMsgPackAndBack(t *testing.T) {
	json := `{"name":{"first":"Jane","last":"Prichard"},"age":46,"friends":["Charlie", "Vihaan", "Carol"]}`
	r := transutil.JSONToMsgPack(bytes.NewBufferString(json))
	data, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	r = transutil.MsgPackToJSON(transutil.JSONToMsgPack(bytes.NewBufferString(json)))
	data, err = ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !matchingJSON(json, string(data)) {
		t.Fatal("json mismatch")
	}
}

func TestJSONUglyToPrettyAndBack(t *testing.T) {
	json := `{"name":"Jane"}`
	r := transutil.JSONToPrettyJSON(bytes.NewBufferString(json))
	data, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\n  \"name\": \"Jane\"\n}" {
		t.Fatal("mismatch")
	}
	r = transutil.JSONToUglyJSON(transutil.JSONToPrettyJSON(bytes.NewBufferString(json)))
	data, err = ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != json {
		t.Fatal("mismatch")
	}
}

func TestJSONToProtoAndBackOne(t *testing.T) {
	var pb pbtest.Test
	json := `{"label":"hello","type":17,"reps":["1","2","3","4"],"optionalgroup":{"requiredField":"good bye"}}`
	r := transutil.ProtoBufToJSON(transutil.JSONToProtoBuf(bytes.NewBufferString(json), &pb, false), &pb, false)
	data, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !matchingJSON(string(data), json) {
		t.Fatal("not matching")
	}
}

func TestJSONToProtoAndBackMulti(t *testing.T) {
	var pb pbtest.Test
	var json string
	json += `{"label":"hello","type":17,"reps":["1","2","3","4"],"optionalgroup":{"requiredField":"good bye"}}`
	json += `{"label":"hola","type":17,"reps":["5","6","7","8"],"optionalgroup":{"requiredField":"adios"}}`
	json += `{"label":"aloha","type":17,"reps":["9","10","11","12"],"optionalgroup":{"requiredField":"aloha"}}`
	r := transutil.ProtoBufToJSON(transutil.JSONToProtoBuf(bytes.NewBufferString(json), &pb, true), &pb, true)
	data, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !matchingJSON(string(data), json) {
		t.Fatal("not matching")
	}
}
