package typeid

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"github.com/speps/go-hashids/v2"
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
const slugSize = 7
const slugNumBytes = 4
const suffixNumBytes = 12
const suffixBase = 62

// Creates a hashid coder setup for slugs:
// - Base58 alphabet
// - Min length of 7
func hashidCoder() (*hashids.HashID, error) {
	h, err := hashids.NewWithData(&hashids.HashIDData{
		Alphabet: base58Alphabet,
		// Technically an unsinged 32-bit int in base58 is 6 characters long.
		// Hashid sometimes ends up in 7 characters as a result of trying to avoid
		// curse words. Note that this also doesn't follow the strict definition of
		// base58 encoding.
		MinLength: slugSize,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return h, nil
}

// Encodes the first 4 bytes (an unsigned 32-bit integer) as a slug.
func encodeSlug(bytes []byte) string {
	encoder, err := hashidCoder()
	if err != nil {
		panic(err)
	}
	i := binary.BigEndian.Uint32(bytes)
	encoding, err := encoder.EncodeInt64([]int64{int64(i)})
	if err != nil {
		panic(err)
	}
	return encoding
}

func decodeSlug(data string) ([]byte, error) {
	decoder, err := hashidCoder()
	if err != nil {
		panic(err)
	}
	decoding, err := decoder.DecodeInt64WithError(data)
	if err != nil {
		panic(err)
	}
	bytes := make([]byte, slugNumBytes)
	binary.BigEndian.PutUint32(bytes, uint32(decoding[0]))
	return bytes, nil
}

func encodeSuffix(data []byte) string {
	bigInt := &big.Int{}
	bigInt.SetBytes(data)
	return fmt.Sprintf("%017s", bigInt.Text(suffixBase))
}

func decodeSuffix(data string) []byte {
	bigInt := &big.Int{}
	bigInt.SetString(data, suffixBase)
	bytes := make([]byte, suffixNumBytes)
	bigInt.FillBytes(bytes)
	return bytes
}
