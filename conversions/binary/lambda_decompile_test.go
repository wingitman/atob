package binary

import (
	"testing"
)

func TestBeautifyJS(t *testing.T) {
	minified := `function handler(event){var body=JSON.parse(event.body);if(body.status==="ok"){return{statusCode:200,body:"Success"};}else{return{statusCode:400,body:"Error"};}}`
	expected := `function handler(event) {
  var body=JSON.parse(event.body);
  if(body.status==="ok") {
    return {
      statusCode: 200, body: "Success"
    }
    ;
  }
  else {
    return {
      statusCode: 400, body: "Error"
    }
    ;
  }
}`

	result := beautifyJS(minified)
	if result != expected {
		t.Errorf("JS beautifier mismatch.\nGot:\n%s\n\nWant:\n%s", result, expected)
	}
}

func TestParseJavaDescriptor(t *testing.T) {
	tests := []struct {
		desc string
		want string
	}{
		{"I", "int"},
		{"Z", "boolean"},
		{"V", "void"},
		{"Ljava/lang/String;", "java.lang.String"},
		{"[Ljava/lang/String;", "java.lang.String[]"},
		{"[[I", "int[][]"},
	}

	for _, tt := range tests {
		got := parseJavaDescriptor(tt.desc)
		if got != tt.want {
			t.Errorf("parseJavaDescriptor(%q) = %q, want %q", tt.desc, got, tt.want)
		}
	}
}

func TestParseJavaMethodDescriptor(t *testing.T) {
	desc := "(Ljava/lang/Object;Lcom/amazonaws/services/lambda/runtime/Context;)Ljava/lang/Object;"
	wantArgs := []string{"java.lang.Object", "com.amazonaws.services.lambda.runtime.Context"}
	wantRet := "java.lang.Object"

	args, ret := parseJavaMethodDescriptor(desc)
	if ret != wantRet {
		t.Errorf("parseJavaMethodDescriptor return type = %q, want %q", ret, wantRet)
	}

	if len(args) != len(wantArgs) {
		t.Fatalf("parseJavaMethodDescriptor got %d args, want %d", len(args), len(wantArgs))
	}

	for i, arg := range args {
		if arg != wantArgs[i] {
			t.Errorf("arg[%d] = %q, want %q", i, arg, wantArgs[i])
		}
	}
}

func TestLookupPythonVersion(t *testing.T) {
	tests := []struct {
		magic uint32
		want  string
	}{
		{3394, "Python 3.6"},
		{3413, "Python 3.7"},
		{3414, "Python 3.8"},
		{3425, "Python 3.9"},
		{3439, "Python 3.10"},
		{3515, "Python 3.11"},
		{3531, "Python 3.12"},
		{3550, "Python 3.13"},
	}

	for _, tt := range tests {
		got := lookupPythonVersion(tt.magic)
		if got != tt.want {
			t.Errorf("lookupPythonVersion(%d) = %q, want %q", tt.magic, got, tt.want)
		}
	}
}

func TestDecompileConvertBytesFallback(t *testing.T) {
	c := lambdaDecompile{}
	out, err := c.ConvertBytes([]byte("invalid-random-data"))
	if err != nil {
		t.Fatalf("ConvertBytes on invalid bytes returned error: %v", err)
	}
	if !testing.Short() {
		// Just check that it gives a helpful decompile error message
		t.Log(out)
	}
}

func TestReadCompressedInt(t *testing.T) {
	tests := []struct {
		input []byte
		want  uint32
	}{
		{[]byte{0x03}, 3},
		{[]byte{0x80 | 0x1f, 0xff}, 0x1fff},
		{[]byte{0xC0 | 0x01, 0x02, 0x03, 0x04}, 0x1020304},
	}

	for _, tt := range tests {
		got, _ := readCompressedInt(tt.input)
		if got != tt.want {
			t.Errorf("readCompressedInt(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestDecodeDotNetType(t *testing.T) {
	tests := []struct {
		input []byte
		want  string
	}{
		{[]byte{0x01}, "void"},
		{[]byte{0x08}, "int"},
		{[]byte{0x0e}, "string"},
		{[]byte{0x1c}, "object"},
		{[]byte{0x1d, 0x08}, "int[]"},
	}

	for _, tt := range tests {
		got, _ := decodeDotNetType(tt.input)
		if got != tt.want {
			t.Errorf("decodeDotNetType(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

