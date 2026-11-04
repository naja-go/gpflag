package gpflag

import (
	"encoding/base64"
	"fmt"
	"os"
	"testing"
)

func setUpBytesHex(bytesHex *[]byte) (*FlagSet, *Flag[[]byte]) {
	f := NewFlagSet("test", ContinueOnError)
	flag := f.NewVar(bytesHex, "bytes", []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}, "Some bytes in HEX")
	f.NewVar(bytesHex, "bytes2", []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}, "Some bytes in HEX").Short('B')
	return f, flag
}

func TestBytesHex(t *testing.T) {
	testCases := []struct {
		input    string
		success  bool
		expected string
	}{
		/// Positive cases
		{"", true, ""}, // Is empty string OK ?
		{"01", true, "01"},
		{"0101", true, "0101"},
		{"1234567890abcdef", true, "1234567890ABCDEF"},
		{"1234567890ABCDEF", true, "1234567890ABCDEF"},

		// Negative cases
		{"0", false, ""},   // Short string
		{"000", false, ""}, /// Odd-length string
		{"qq", false, ""},  /// non-hex character
	}

	devnull, _ := os.Open(os.DevNull)
	os.Stderr = devnull

	for i := range testCases {
		var bytesHex []byte
		f, flag := setUpBytesHex(&bytesHex)

		tc := &testCases[i]

		// --bytes
		args := []string{
			fmt.Sprintf("--bytes=%s", tc.input),
			fmt.Sprintf("-B  %s", tc.input),
			fmt.Sprintf("--bytes2=%s", tc.input),
		}

		for _, arg := range args {
			err := f.Parse([]string{arg})

			if err != nil && tc.success == true {
				t.Errorf("expected success, got %q", err)
				continue
			} else if err == nil && tc.success == false {
				// bytesHex, err := f.GetBytesHex("bytes")
				t.Errorf("expected failure while processing %q", tc.input)
				continue
			} else if tc.success {
				flag, err = f.Lookup[[]byte]("bytes")
				if err != nil {
					t.Fatal("Lookup[[]byte]:", err)
				}
				bytesHex := flag.Get()
				if fmt.Sprintf("%X", bytesHex) != tc.expected {
					t.Errorf("expected %q, got '%X'", tc.expected, bytesHex)
				}
			}
		}
	}
}

func setUpBytesBase64(bytesBase64 *[]byte) (*FlagSet, *Flag[[]byte]) {
	f := NewFlagSet("test", ContinueOnError)
	flag := f.NewVar((*[]byte)(bytesBase64), "bytes", []byte([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}), "Some bytes in Base64", Base64())
	f.NewVar((*[]byte)(bytesBase64), "bytes2", []byte([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}), "Some bytes in Base64", Base64()).Short('B')
	return f, flag
}

func TestBytesBase64(t *testing.T) {
	testCases := []struct {
		input    string
		success  bool
		expected string
	}{
		/// Positive cases
		{"", true, ""}, // Is empty string OK ?
		{"AQ==", true, "AQ=="},

		// Negative cases
		{"AQ", false, ""}, // Padding removed
		{"ï", false, ""},  // non-base64 characters
	}

	devnull, _ := os.Open(os.DevNull)
	os.Stderr = devnull

	for i := range testCases {
		var bytesBase64 []byte
		f, flag := setUpBytesBase64(&bytesBase64)

		tc := &testCases[i]

		// --bytes
		args := []string{
			fmt.Sprintf("--bytes=%s", tc.input),
			fmt.Sprintf("-B  %s", tc.input),
			fmt.Sprintf("--bytes2=%s", tc.input),
		}

		for _, arg := range args {
			err := f.Parse([]string{arg})

			if err != nil && tc.success == true {
				t.Errorf("expected success, got %q", err)
				continue
			} else if err == nil && tc.success == false {
				// bytesBase64, err := f.GetBytesBase64("bytes")
				t.Errorf("expected failure while processing %q", tc.input)
				continue
			} else if tc.success {
				flag, err = f.Lookup[[]byte]("bytes")
				if err != nil {
					t.Fatal("Lookup[[]byte]:", err)
				}
				bytesBase64 := flag.Get()
				if base64.StdEncoding.EncodeToString(bytesBase64) != tc.expected {
					t.Errorf("expected %q, got '%X'", tc.expected, bytesBase64)
				}
			}
		}
	}
}
