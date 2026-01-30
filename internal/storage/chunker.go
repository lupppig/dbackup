package storage

import (
	"bufio"
	"io"
)

const (
	minChunkSize = 32 * 1024  // 32KB
	avgChunkSize = 64 * 1024  // 64KB
	maxChunkSize = 512 * 1024 // 512KB
)

// Pre-calculated Gear table with high entropy
var gear = [256]uint64{
	0xd7b65d12b54bd28d, 0xf00de64c4fc2d06b, 0xeab57f300049a495, 0x4f9e3f8aba6e66be,
	0x83126ffbac69f837, 0xe5d0a39f56f113da, 0x1ac4f087dade45b9, 0xdafa3b53e6bf16bd,
	0xe8920f66ad6a4c4c, 0x026ac30cbd60cc63, 0xcf4edba422741c61, 0x061a39569362ce20,
	0x31187ff7532ef733, 0xda17a3872ed072fa, 0xfee8a65e3d0e6ddc, 0xaed6152e5909c925,
	0x3bab09a86f2ba817, 0xd9d53f1194650680, 0x0dc6cbf59ffeac43, 0x469d9215148278fd,
	0xadf8b4150f31b2b2, 0x9f4e2452671ddcf8, 0xd132d95f3e271e11, 0xbcd03e33970d16b7,
	0x629d2ee1ea512cf1, 0x294f9ac3aa4a9eb7, 0x21aefbcdccb63ef9, 0x67c465bbba7fca84,
	0x23ce88cf318de6e3, 0xccd046aaa9da3032, 0x6422dc56a05c7814, 0x78591cc0401b9499,
	0xcb910e011e05d7f3, 0xa9272ab09fb88011, 0xa1175a6797528b9b, 0x5e5c2adebff59d61,
	0xfe88a021607f22fc, 0x0e9b3ce544af6e24, 0xf8e872357ee7b069, 0xc8ce03b9f8dd3646,
	0x27de9c5be54fb01f, 0x77d3d98783778a93, 0xce76fbd69edf895e, 0x39e0dd7daa9cd108,
	0xa25ebbfb8456438a, 0xe71a9682731566fc, 0xabff6df888c6cee2, 0xab2c195e627959bf,
	0x03ceafb768cf56f0, 0x1cf6823d7ce82b4c, 0x76d6518995d16223, 0x8038be1058e99e4f,
	0xe24d7a019a0f3ef6, 0xb4fb43b9ce0b052e, 0xe16eb95ec5857071, 0xc716fee15de352d1,
	0xe7698f7be4fcd391, 0xc920f20d9b055dd9, 0xc5c874636cbcc288, 0x8269d090288fdb17,
	0x33cd732317388ce0, 0x76d7c54a49d779b4, 0xefcfbf579f2ca5c9, 0x57bb71402a2ff330,
	0x535d27942c3a1fe5, 0x6da5f13e7ba0693d, 0x970ad80e12a5418d, 0x7789f5d2bcc21bf0,
	0x9772ad4a53deabf8, 0xa67cd4c3f13b8fb3, 0x99e69942194f4424, 0xefb5b0f0d7546707,
	0xaa75b6ed9f8b9934, 0x7fea08f4e1c20c0d, 0x386bb00b3df5960a, 0xdeee28c9b277df0f,
	0xab11e7fb3d8fb1aa, 0x43688f65ce082564, 0x046ec94eb3447e86, 0x61627f868e4dca7a,
	0x5f1899ebe004ee65, 0x11ca7f09a5c49534, 0x5084ee91fc65c169, 0x2db765b3d4e20b56,
	0xaff3bd1f455f75f8, 0x9b5ef9f7ea3f0785, 0x083ddddeb8c418b8, 0x99e72a3cc9ec6c3d,
	0xa61f0cfaea602120, 0xd0e4cf3321f6caaf, 0x047a75a0025c7e90, 0x66e1eb4ba403de9e,
	0xef90350cc764677e, 0x9bea389ec9c578c4, 0x6b0e0b1e2d815463, 0x352433d6f020a005,
	0xb7a75471c291dac9, 0x7559a53e098873e1, 0x1f87c5fa1ba03282, 0x50c6c99cb84d547c,
	0xc7bbb793e8e5fc5e, 0x3ede1928ae286bd9, 0x6d110f7f422276cf, 0xf9a688dc0628112e,
	0x0bbef931e9fff636, 0xb97a43d1cb28e75f, 0x31d6c11de035091f, 0xd215b780ccc966f8,
	0x42e413358aa53645, 0xfce134323e33a211, 0x69ef4d364db4db42, 0x120cf983abcf6d6e,
	0x8f9696a5b5461948, 0xee7b98674ca565f2, 0x0e6551fcf957853d, 0x798a43544c206ba1,
	0xfe6be5895a094956, 0x0cfb2671cf129c58, 0x4c49c8cd78b4df37, 0x10b55917b6a7634e,
	0x8a28b3964333f6f4, 0x63be2ba83ef3acee, 0x18093e73de3c5786, 0xa9eae1fdfcbc5bd3,
	0xcb3a11bcdf11023f, 0x9713bb1ad3e9567c, 0x6589197a220b84ad, 0xc2e20e12c006d09d,
	0x573eaef8c8067ca1, 0x29446be56e9d0f12, 0x08f7216be896c59a, 0xe0cde76197c757e8,
	0x51e6cf1b005b6910, 0x085c435bfeff4383, 0x85cb0b55c440df64, 0x8aac9e0e88115d4d,
	0x0740ec1dacd38fc0, 0x58721517dbc0b953, 0xf5ea79838b6bb780, 0xce00e195dfe41680,
	0x81a84a9fa93d1dbf, 0x80076ec6bc346c1f, 0xeb3091e9c5ef182b, 0x04b861c65a722112,
	0xc35d69743c2a6788, 0x7db29c84f05b8d76, 0xcd35532c55fdab00, 0x2e1bdda00d1eae50,
	0xa173cd0346a607de, 0xac8dbdcc315b281b, 0x29999281428863a9, 0x6cb2df750f125dd4,
	0xb06bda7112749ecd, 0x11a92bf7c139586d, 0x0bb33b8459117c90, 0x18653e63eecb44ae,
	0x197f7bc6e4020540, 0x6ec5094e5ebe786f, 0x45d9e745583a2e59, 0x6537cb14c9ce9294,
	0xe7f3ec0c61872455, 0x6f768e13e8baa5cf, 0xd704f7e17cb53caf, 0x53e32eed0a85bb4d,
	0xaeb6e207e7921ab3, 0xa52f33431a5858ee, 0xfbd795d4e342f1f5, 0xeb80b14350fec904,
	0x882ad4391d76a906, 0x38832a6f1cc761e3, 0x4c2db2b701b18a84, 0x8b7139d94cc74df5,
	0x26879d6a5abfd1a8, 0x5d3f2bf9af234bd0, 0x359d4c8830e7ef4f, 0xdc886889f045cf29,
	0x3ff4e0633eeb4072, 0x1d6e50c425c28f56, 0x42d4c7d112490553, 0xe110ab93b6ac729d,
	0xd27aae501bde1696, 0xd703453cf2954336, 0x6b7d7a74fac8692e, 0xf66da28d87cd0001,
	0xda3e6162b6eec550, 0x496c9de0fe273ee6, 0xf05f583fa2f2143c, 0x955e67c7fe82d560,
	0xc3263deafcbf4f2c, 0xb8c983480b388010, 0x93d64a64be2d8961, 0x74ac2888da8f4e73,
	0xd570f28281cfad04, 0x40512b1eaec561a2, 0x55502dc63fbdba2f, 0xddab136b6c4940d6,
	0x090c4ee4f3b9cb5c, 0x48823c166921e79b, 0x015b13f2edfebca7, 0x06ae83e5fd9d4f26,
	0xf5706815991fabc6, 0x24dd191d22c532b3, 0x981e192ca7ba256d, 0xc8bdafd77f94210c,
	0x6fa3a9f4b9f38830, 0x0153986a4f89e435, 0x68664696458d4ce7, 0x994e8ad943e4baa0,
	0xdf8d6dc9dded3a8f, 0x327702e95a679286, 0x64da29d7712dd2d8, 0x66ace32cb99ba6d9,
	0xed2842b43135af54, 0x58789f49fd96dfd3, 0xbdeffeb25a2ac8b2, 0x7a754bfe6d7b3c04,
	0x050dcfefaf0d7d55, 0xecb232837253b850, 0x9eb792ed303015bb, 0xa561a692af2e6d10,
	0x589d66d930d28414, 0x7df9d2ff8722db41, 0xac126e4846b0d9cc, 0x24fb1871a1229785,
	0xdfba3d634dc54aff, 0x018366533a04e21e, 0xdd49a7908f5e32bf, 0x79c0d758adea3a7c,
	0xdac00719d057a13d, 0xf1c9a2dc421cdb8f, 0xc2b275fcbbcbecff, 0xa13a0aa0fd5f5c78,
	0xc1f4e7f4453daf09, 0xd9dfa5fa2ad1b3bb, 0xb19a94af15378a25, 0x6bdf61442cdda8e6,
	0xf0c2262ed7f6669e, 0xabd523a78e186565, 0x77d91acbec7ae15d, 0x01a8a5ec64cab7eb,
	0xabf90ece8abd70b9, 0xf1a28bf8d68883b3, 0x345c9f0bf5272d83, 0xcafe4c0885602f90,
	0x5ad8fcb3892fef18, 0xa20598c2eebe8b5f, 0x3fe3f577f57e303c, 0x77884730dd1e3b8f,
	0x971f42c8dcfe2e20, 0xfa04116074359ad0, 0x580541a4c5f9d699, 0x77815299e748c4f1,
	0x6efebc314aef9fd5, 0x85081a33cc5a0e89, 0x774d6226ce259c35, 0xc645ee032ad5c172,
}

type Chunker struct {
	r *bufio.Reader
}

func NewChunker(r io.Reader) *Chunker {
	return &Chunker{r: bufio.NewReader(r)}
}

// Next returns the next content-defined chunk.
func (c *Chunker) Next() ([]byte, error) {
	var buf []byte
	var hash uint64

	// 1. Read minimum chunk size
	for len(buf) < minChunkSize {
		b, err := c.r.ReadByte()
		if err != nil {
			if len(buf) > 0 {
				return buf, nil
			}
			return nil, err
		}
		buf = append(buf, b)
		hash = (hash << 1) ^ gear[b]
	}

	// 2. Scan for boundary using rolling hash
	// Mask for ~16KB average
	mask := uint64(0x3FFF)

	for len(buf) < maxChunkSize {
		b, err := c.r.ReadByte()
		if err != nil {
			return buf, nil
		}
		buf = append(buf, b)
		hash = (hash << 1) ^ gear[b]

		if (hash & mask) == 0 {
			break
		}
	}

	return buf, nil
}
