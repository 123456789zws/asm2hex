package archs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suifei/asm2hex/bindings/keystone"
)

func TestLooksLikeAssembly(t *testing.T) {
	assert.True(t, looksLikeAssembly("mov x0, #1"))
	assert.True(t, looksLikeAssembly("str x9, [fp, #-0x10]"))
	assert.True(t, looksLikeAssembly("movk x9, #0x6974, lsl #16"))
	assert.False(t, looksLikeAssembly("[fp, #-0x10]"))
	assert.False(t, looksLikeAssembly("[x0]"))
	assert.False(t, looksLikeAssembly("#0x10"))
	assert.False(t, looksLikeAssembly(""))
	assert.False(t, looksLikeAssembly("  "))
}

func TestSupportsKeystoneSyntax(t *testing.T) {
	assert.True(t, supportsKeystoneSyntax(keystone.ARCH_X86))
	assert.False(t, supportsKeystoneSyntax(keystone.ARCH_ARM64))
	assert.False(t, supportsKeystoneSyntax(keystone.ARCH_ARM))
	assert.False(t, supportsKeystoneSyntax(keystone.ARCH_MIPS))
}

func TestAssemble_ARM64_movk_shift(t *testing.T) {
	// Issue #8: movk with lsl must assemble when syntax option is not applied.
	encoding, count, ok, err := Assemble(
		keystone.ARCH_ARM64,
		keystone.MODE_LITTLE_ENDIAN,
		"movk x9, #0x6974, lsl #16",
		0,
		false,
		-1, // no syntax option
	)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, uint64(1), count)
	require.Equal(t, []byte{0x89, 0x2e, 0xad, 0xf2}, encoding)
}

func TestAssemble_ARM64_movk_with_invalid_syntax_option_ignored(t *testing.T) {
	// Even if UI still passes OPT_SYNTAX_INTEL for ARM64, Assemble must ignore it.
	encoding, count, ok, err := Assemble(
		keystone.ARCH_ARM64,
		keystone.MODE_LITTLE_ENDIAN,
		"movk x9, #0x6974, lsl #16",
		0,
		false,
		int(keystone.OPT_SYNTAX_INTEL),
	)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, uint64(1), count)
	require.Equal(t, []byte{0x89, 0x2e, 0xad, 0xf2}, encoding)
}

func TestAssemble_rejects_operand_fragment(t *testing.T) {
	// Issue #7: bare memory operand fragments must not be sent to Keystone.
	_, _, ok, err := Assemble(
		keystone.ARCH_ARM64,
		keystone.MODE_LITTLE_ENDIAN,
		"[fp, #-0x10]",
		0,
		false,
		-1,
	)
	require.False(t, ok)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid assembly")
}

func TestAssemble_ARM64_issue8_sample(t *testing.T) {
	lines := []string{
		"mov        x9, #0x6341",
		"movk       x9, #0x6974, lsl #16",
		"movk       x9, #0x6176, lsl #32",
		"movk       x9, #0x6574, lsl #48",
		"str        x9, [x8]",
		"mov        x9, #0x2064",
		"movk       x9, #0x6576, lsl #16",
		"movk       x9, #0x7372, lsl #32",
		"movk       x9, #0x6f69, lsl #48",
		"str        x9, [x8, #8]",
		"mov        w9, #0x6e",
		"strb       w9, [x8, #16]",
		"mov        w9, #17",
		"strb       w9, [x8, #23]",
		"ret",
	}
	want := []string{
		"29688CD2",
		"892EADF2",
		"C92ECCF2",
		"89AEECF2",
		"090100F9",
		"890C84D2",
		"C9AEACF2",
		"496ECEF2",
		"29EDEDF2",
		"090500F9",
		"C90D8052",
		"09410039",
		"29028052",
		"095D0039",
		"C0035FD6",
	}
	for i, line := range lines {
		encoding, _, ok, err := Assemble(keystone.ARCH_ARM64, keystone.MODE_LITTLE_ENDIAN, line, 0, false, -1)
		require.NoError(t, err, "line %d: %s", i, line)
		require.True(t, ok, "line %d: %s", i, line)
		got := ""
		for _, b := range encoding {
			got += sprintf02X(b)
		}
		require.Equal(t, want[i], got, "line %d: %s", i, line)
	}
}

func sprintf02X(b byte) string {
	const hexdigits = "0123456789ABCDEF"
	return string([]byte{hexdigits[b>>4], hexdigits[b&0x0f]})
}

func TestDisassemble(t *testing.T) {
	// Capstone CGO binding can crash (heap corruption) with mismatched
	// shared libraries in some environments; keep the test as a compile check only.
	t.Skip("capstone native smoke test disabled (environment-dependent CGO)")
}

func TestAssemble_x86_64_code_little_endian(t *testing.T) {
	code := "mov rax, 1"
	encoding, count, ok, err := Assemble(keystone.ARCH_X86, keystone.MODE_64, code, 0x100, false, int(keystone.OPT_SYNTAX_INTEL))
	assert.Nil(t, err)
	assert.Equal(t, uint64(1), count)
	assert.True(t, ok)
	assert.Equal(t, []byte{0x48, 0xc7, 0xc0, 0x01, 0x00, 0x00, 0x00}, encoding)
}

func TestAssemble_ARM_code_little_endian(t *testing.T) {
	code := "mov r1, #1"
	encoding, count, ok, err := Assemble(keystone.ARCH_ARM, keystone.MODE_ARM, code, 0x100, false, -1)
	assert.Nil(t, err)
	assert.Equal(t, uint64(1), count)
	assert.True(t, ok)
	assert.Equal(t, []byte{0x01, 0x10, 0xa0, 0xe3}, encoding)
}

func TestAssemble_ARM64_code_little_endian(t *testing.T) {
	code := "mov w1, #1"
	encoding, count, ok, err := Assemble(keystone.ARCH_ARM64, keystone.MODE_LITTLE_ENDIAN, code, 0x100, false, -1)
	assert.Nil(t, err)
	assert.Equal(t, uint64(1), count)
	assert.True(t, ok)
	assert.Equal(t, []byte{0x21, 0x00, 0x80, 0x52}, encoding)
}
