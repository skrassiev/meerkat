package feed

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_StorageDir(t *testing.T) {
	assert.Equal(t, datadirUsermode, getStorageDir())
}

func Test_StorageDirAsFilePath(t *testing.T) {
	// this will likely not work in the concurrent tests exceution
	oldDir := datadirUsermode
	defer func() {
		datadirUsermode = oldDir
	}()

	datadirUsermode = "foodir/bar"
	os.RemoveAll("./foodir")
	err := os.Mkdir("./foodir", 0755)
	assert.NoError(t, err)

	f, err := os.Create(datadirUsermode)
	assert.NoError(t, err)
	assert.NotNil(t, f)
	f.Close()

	assert.Equal(t, "", getStorageDir())
}

func testParseIP(t *testing.T) {

}

func Test_StorageStoredIP(t *testing.T) {
	assert.NotEmpty(t, getStorageDir())

	for _, v := range []struct {
		data   []byte
		expect IPv4
	}{
		{data: []byte("asasg sdfdfhsdfhgsdgf d sgsgsgas sa"), expect: ""},
		{data: []byte{}, expect: ""},
		{data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x05, 0x03, 0x06, 0x0a}, expect: ""},
		{data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x05, 0x03, 0x06, 0x0a, 0x12, 0x22, 0x23, 0x03, 0x00, 0x27, 0x15}, expect: ""},
		{data: []byte("192.168.0"), expect: ""},
		{data: []byte("192.168.0.1"), expect: "192.168.0.1"},
		{data: []byte("192.267.0.1"), expect: ""},
		{data: []byte("255.255.255.255"), expect: "255.255.255.255"},
	} {
		ipSrc := IPv4(string(v.data))
		ipSrc.write()
		var ip IPv4
		ip.read()
		assert.Equal(t, v.expect, ip, "unexpected IP value for %s", string(v.data))
	}
}
