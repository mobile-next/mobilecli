package daemon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFrameAppendsNewline(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, writeFrame(&buf, map[string]int{"a": 1}))
	assert.Equal(t, "{\"a\":1}\n", buf.String())
}

func TestReadFrameHandlesLinesLargerThanBufioDefault(t *testing.T) {
	big := strings.Repeat("x", 200_000)
	var buf bytes.Buffer
	require.NoError(t, writeFrame(&buf, map[string]string{"data": big}))

	line, err := readFrame(bufio.NewReader(&buf))
	require.NoError(t, err)

	var decoded map[string]string
	require.NoError(t, json.Unmarshal(line, &decoded))
	assert.Equal(t, big, decoded["data"])
}

func TestReadFrameReturnsErrorOnEOFWithoutData(t *testing.T) {
	_, err := readFrame(bufio.NewReader(strings.NewReader("")))
	require.Error(t, err)
}

func TestResponseErrorUnmarshalsIntoRPCError(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"Server error","data":"device not found"}}`
	var resp response
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32000, resp.Error.Code)
	assert.Equal(t, "device not found", resp.Error.Error())
}
