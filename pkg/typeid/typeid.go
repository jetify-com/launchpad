package typeid

import (
	"fmt"
	"regexp"

	"github.com/google/uuid"
)

const SlugLength = 7

type TypeID struct {
	typeStr   string
	slugStr   string
	suffixStr string
	value     [16]byte
}

var typeIDRegex = regexp.MustCompile(`([a-zA-Z0-9]+)_([a-zA-Z0-9]{22,24})`)

// Generates a random TypeID with the given type prefix.
func New(typePrefix string) *TypeID {
	uid := uuid.New()

	id := &TypeID{
		typeStr: typePrefix, // TODO: validate typePrefix format
		value:   uid,
	}
	return id
}

// Returns a typeID given a string.
// String should be in the format of ([a-zA-Z0-9]+)_([a-zA-Z0-9]+)
func FromString(idStr string) *TypeID {
	matches := typeIDRegex.FindAllStringSubmatch(idStr, -1)
	if len(matches) > 0 {
		match := matches[0] // match is in the format of [typeIDstr, prefix, theRest]
		return &TypeID{
			typeStr:   match[1],
			slugStr:   match[2][0:SlugLength],
			suffixStr: match[2][SlugLength:],
			// TODO: decode value field.
		}
	}

	// idStr is in the wrong format
	return nil
}

// Returns the id portion of the TypeID (without the type prefix).
// The random bytes are a valid UUID v4.
func (id *TypeID) Bytes() []byte {
	return id.value[:]
}

// Returns the type prefix
func (id *TypeID) Type() string {
	return id.typeStr
}

// Returns a 7-character slug that is URL friendly.
//
// Slugs meet the following properties:
// - They are 7 characters long
// - The slug is a prefix of the random character portion of a TypeID.
// - Slugs use the base58 character set, in order to avoid confusion between similar characters.
// - Some effort is put to avoid the most common curse words.
func (id *TypeID) Slug() string {
	if id.slugStr == "" {
		// Encode the first 4 bytes as a slug
		id.slugStr = encodeSlug(id.value[:slugNumBytes])
	}
	return id.slugStr
}

// The non-slug bytes of the id, encoded in base62.
// We fix the size to 17 characters, and pad small values with zeros.
func (id *TypeID) suffix() string {
	if id.suffixStr == "" {
		// Skip the first 4 bytes that are reserved for the slug
		id.suffixStr = encodeSuffix(id.value[slugNumBytes:])
	}
	return id.suffixStr
}

// The TypeID in string format.
//
// TypeIDs have the following format:
//
//	prefix_1234567abcdefghijklmnopq
//
// Where:
//   - prefix is prefix suffix specified at creation time
//   - '_' is used as a separator between the type prefix and the random character of the id.
//   - 1234567 are seven characters that can be used as a slug, from the base58 character set.
//   - abcdefghijklmnopq are 17 random characters from the base62 character set.
func (id *TypeID) String() string {
	return fmt.Sprintf("%s_%s%s", id.Type(), id.Slug(), id.suffix())
}
