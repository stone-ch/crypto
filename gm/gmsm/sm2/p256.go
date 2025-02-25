/*
Copyright Suzhou Tongji Fintech Research Institute 2017 All Rights Reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sm2

import (
	"crypto/elliptic"
	"math/big"
	"sync"
	"unsafe"
)

/**
 * This is an optimized sm2-p256 implementation.
 *
 * NOTE from the previous authors
 * 学习标准库p256的优化方法实现sm2的快速版本
 * 标准库的p256的代码实现有些晦涩难懂，当然sm2的同样如此，有兴趣的大家可以研究研究，最后神兽压阵。。。
 *
 * ━━━━━━animal━━━━━━
 * 　　　┏┓　　　┏┓
 * 　　┏┛┻━━━┛┻┓
 * 　　┃　　　　　　　┃
 * 　　┃　　　━　　　┃
 * 　　┃　┳┛　┗┳　┃
 * 　　┃　　　　　　　┃
 * 　　┃　　　┻　　　┃
 * 　　┃　　　　　　　┃
 * 　　┗━┓　　　┏━┛
 * 　　　┃　　　┃
 *　　 　┃　　　┃
 *　　　 ┃　　　┗━━━┓
 *	   　┃　　　　　┣┓
 *   　　┃　　　　　┏┛
 *　　 　┗┓┓┏━┳┓┏┛
 *　　　　┃┫┫ ┃┫┫
 *　　　　┗┻┛ ┗┻┛
 *
 * ━━━━━Kawaii ━━━━━━
 */

type sm2P256Curve struct {
	RInverse *big.Int
	*elliptic.CurveParams
	a, b, gx, gy sm2P256FieldElement
}

var initonce sync.Once
var sm2P256 sm2P256Curve

type sm2P256FieldElement [9]uint32
type sm2P256LargeFieldElement [17]uint64

const (
	bottom28BitsMask = 0xFFFFFFF
	bottom29BitsMask = 0x1FFFFFFF
	bottom32BitsMask = 0xFFFFFFFF
	bottom57BitsMask = 0x1FFFFFFFFFFFFFF
	twoPower57       = 0x200000000000000
)

func initP256Sm2() {
	sm2P256.CurveParams = &elliptic.CurveParams{Name: "SM2-P-256"} // sm2
	A, _ := new(big.Int).SetString("FFFFFFFEFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00000000FFFFFFFFFFFFFFFC", 16)
	//SM2椭	椭 圆 曲 线 公 钥 密 码 算 法 推 荐 曲 线 参 数
	sm2P256.P, _ = new(big.Int).SetString("FFFFFFFEFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00000000FFFFFFFFFFFFFFFF", 16)
	sm2P256.N, _ = new(big.Int).SetString("FFFFFFFEFFFFFFFFFFFFFFFFFFFFFFFF7203DF6B21C6052B53BBF40939D54123", 16)
	sm2P256.B, _ = new(big.Int).SetString("28E9FA9E9D9F5E344D5A9E4BCF6509A7F39789F515AB8F92DDBCBD414D940E93", 16)
	sm2P256.Gx, _ = new(big.Int).SetString("32C4AE2C1F1981195F9904466A39C9948FE30BBFF2660BE1715A4589334C74C7", 16)
	sm2P256.Gy, _ = new(big.Int).SetString("BC3736A2F4F6779C59BDCEE36B692153D0A9877CC62A474002DF32E52139F0A0", 16)
	sm2P256.RInverse, _ = new(big.Int).SetString("7ffffffd80000002fffffffe000000017ffffffe800000037ffffffc80000002", 16)
	sm2P256.BitSize = 256
	sm2P256FromBig(&sm2P256.a, A)
	sm2P256FromBig(&sm2P256.gx, sm2P256.Gx)
	sm2P256FromBig(&sm2P256.gy, sm2P256.Gy)
	sm2P256FromBig(&sm2P256.b, sm2P256.B)
}

func P256Sm2() elliptic.Curve {
	initonce.Do(initP256Sm2)
	return sm2P256
}

func (curve sm2P256Curve) Params() *elliptic.CurveParams {
	return sm2P256.CurveParams
}

// y^2 = x^3 + ax + b
func (curve sm2P256Curve) IsOnCurve(X, Y *big.Int) bool {
	var a, x, y, y2, x3 sm2P256FieldElement

	sm2P256FromBig(&x, X)
	sm2P256FromBig(&y, Y)

	sm2P256Square2Way(&x3, &x, &y2, &y)

	sm2P256Mul2Way(&x3, &x3, &x, &a, &curve.a, &x)
	sm2P256Add(&x3, &x3, &a)
	sm2P256Add(&x3, &x3, &curve.b)

	return sm2P256ToBig(&x3).Cmp(sm2P256ToBig(&y2)) == 0
}

func (curve sm2P256Curve) ScalarMult(x1, y1 *big.Int, k []byte) (*big.Int, *big.Int) {
	var scalarReversed [32]byte
	var X, Y, Z, X1, Y1 sm2P256FieldElement

	sm2P256FromBig(&X1, x1)
	sm2P256FromBig(&Y1, y1)
	sm2P256GetScalar(&scalarReversed, k)
	sm2P256ScalarMult(&X, &Y, &Z, &X1, &Y1, &scalarReversed)
	return sm2P256ToAffine(&X, &Y, &Z)
}

func (curve sm2P256Curve) ScalarBaseMult(k []byte) (*big.Int, *big.Int) {
	var scalarReversed [32]byte
	var X, Y, Z sm2P256FieldElement

	sm2P256GetScalar(&scalarReversed, k)
	sm2P256ScalarBaseMult(&X, &Y, &Z, &scalarReversed)
	return sm2P256ToAffine(&X, &Y, &Z)
}

var sm2P256Precomputed = [9 * 2 * 16 * 2]uint32{
	0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0,
	0x830053d, 0x328990f, 0x6c04fe1, 0xc0f72e5, 0x1e19f3c, 0x666b093, 0x175a87b, 0xec38276, 0x222cf4b,
	0x185a1bba, 0x354e593, 0x1295fac1, 0xf2bc469, 0x47c60fa, 0xc19b8a9, 0xf63533e, 0x903ae6b, 0xc79acba,
	0x15b061a4, 0x33e020b, 0xdffb34b, 0xfcf2c8, 0x16582e08, 0x262f203, 0xfb34381, 0xa55452, 0x604f0ff,
	0x41f1f90, 0xd64ced2, 0xee377bf, 0x75f05f0, 0x189467ae, 0xe2244e, 0x1e7700e8, 0x3fbc464, 0x9612d2e,
	0x1341b3b8, 0xee84e23, 0x1edfa5b4, 0x14e6030, 0x19e87be9, 0x92f533c, 0x1665d96c, 0x226653e, 0xa238d3e,
	0xf5c62c, 0x95bb7a, 0x1f0e5a41, 0x28789c3, 0x1f251d23, 0x8726609, 0xe918910, 0x8096848, 0xf63d028,
	0x152296a1, 0x9f561a8, 0x14d376fb, 0x898788a, 0x61a95fb, 0xa59466d, 0x159a003d, 0x1ad1698, 0x93cca08,
	0x1b314662, 0x706e006, 0x11ce1e30, 0x97b710, 0x172fbc0d, 0x8f50158, 0x11c7ffe7, 0xd182cce, 0xc6ad9e8,
	0x12ea31b2, 0xc4e4f38, 0x175b0d96, 0xec06337, 0x75a9c12, 0xb001fdf, 0x93e82f5, 0x34607de, 0xb8035ed,
	0x17f97924, 0x75cf9e6, 0xdceaedd, 0x2529924, 0x1a10c5ff, 0xb1a54dc, 0x19464d8, 0x2d1997, 0xde6a110,
	0x1e276ee5, 0x95c510c, 0x1aca7c7a, 0xfe48aca, 0x121ad4d9, 0xe4132c6, 0x8239b9d, 0x40ea9cd, 0x816c7b,
	0x632d7a4, 0xa679813, 0x5911fcf, 0x82b0f7c, 0x57b0ad5, 0xbef65, 0xd541365, 0x7f9921f, 0xc62e7a,
	0x3f4b32d, 0x58e50e1, 0x6427aed, 0xdcdda67, 0xe8c2d3e, 0x6aa54a4, 0x18df4c35, 0x49a6a8e, 0x3cd3d0c,
	0xd7adf2, 0xcbca97, 0x1bda5f2d, 0x3258579, 0x606b1e6, 0x6fc1b5b, 0x1ac27317, 0x503ca16, 0xa677435,
	0x57bc73, 0x3992a42, 0xbab987b, 0xfab25eb, 0x128912a4, 0x90a1dc4, 0x1402d591, 0x9ffbcfc, 0xaa48856,
	0x7a7c2dc, 0xcefd08a, 0x1b29bda6, 0xa785641, 0x16462d8c, 0x76241b7, 0x79b6c3b, 0x204ae18, 0xf41212b,
	0x1f567a4d, 0xd6ce6db, 0xedf1784, 0x111df34, 0x85d7955, 0x55fc189, 0x1b7ae265, 0xf9281ac, 0xded7740,
	0xf19468b, 0x83763bb, 0x8ff7234, 0x3da7df8, 0x9590ac3, 0xdc96f2a, 0x16e44896, 0x7931009, 0x99d5acc,
	0x10f7b842, 0xaef5e84, 0xc0310d7, 0xdebac2c, 0x2a7b137, 0x4342344, 0x19633649, 0x3a10624, 0x4b4cb56,
	0x1d809c59, 0xac007f, 0x1f0f4bcd, 0xa1ab06e, 0xc5042cf, 0x82c0c77, 0x76c7563, 0x22c30f3, 0x3bf1568,
	0x7a895be, 0xfcca554, 0x12e90e4c, 0x7b4ab5f, 0x13aeb76b, 0x5887e2c, 0x1d7fe1e3, 0x908c8e3, 0x95800ee,
	0xb36bd54, 0xf08905d, 0x4e73ae8, 0xf5a7e48, 0xa67cb0, 0x50e1067, 0x1b944a0a, 0xf29c83a, 0xb23cfb9,
	0xbe1db1, 0x54de6e8, 0xd4707f2, 0x8ebcc2d, 0x2c77056, 0x1568ce4, 0x15fcc849, 0x4069712, 0xe2ed85f,
	0x2c5ff09, 0x42a6929, 0x628e7ea, 0xbd5b355, 0xaf0bd79, 0xaa03699, 0xdb99816, 0x4379cef, 0x81d57b,
	0x11237f01, 0xe2a820b, 0xfd53b95, 0x6beb5ee, 0x1aeb790c, 0xe470d53, 0x2c2cfee, 0x1c1d8d8, 0xa520fc4,
	0x1518e034, 0xa584dd4, 0x29e572b, 0xd4594fc, 0x141a8f6f, 0x8dfccf3, 0x5d20ba3, 0x2eb60c3, 0x9f16eb0,
	0x11cec356, 0xf039f84, 0x1b0990c1, 0xc91e526, 0x10b65bae, 0xf0616e8, 0x173fa3ff, 0xec8ccf9, 0xbe32790,
	0x11da3e79, 0xe2f35c7, 0x908875c, 0xdacf7bd, 0x538c165, 0x8d1487f, 0x7c31aed, 0x21af228, 0x7e1689d,
	0xdfc23ca, 0x24f15dc, 0x25ef3c4, 0x35248cd, 0x99a0f43, 0xa4b6ecc, 0xd066b3, 0x2481152, 0x37a7688,
	0x15a444b6, 0xb62300c, 0x4b841b, 0xa655e79, 0xd53226d, 0xbeb348a, 0x127f3c2, 0xb989247, 0x71a277d,
	0x19e9dfcb, 0xb8f92d0, 0xe2d226c, 0x390a8b0, 0x183cc462, 0x7bd8167, 0x1f32a552, 0x5e02db4, 0xa146ee9,
	0x1a003957, 0x1c95f61, 0x1eeec155, 0x26f811f, 0xf9596ba, 0x3082bfb, 0x96df083, 0x3e3a289, 0x7e2d8be,
	0x157a63e0, 0x99b8941, 0x1da7d345, 0xcc6cd0, 0x10beed9a, 0x48e83c0, 0x13aa2e25, 0x7cad710, 0x4029988,
	0x13dfa9dd, 0xb94f884, 0x1f4adfef, 0xb88543, 0x16f5f8dc, 0xa6a67f4, 0x14e274e2, 0x5e56cf4, 0x2f24ef,
	0x1e9ef967, 0xfe09bad, 0xfe079b3, 0xcc0ae9e, 0xb3edf6d, 0x3e961bc, 0x130d7831, 0x31043d6, 0xba986f9,
	0x1d28055, 0x65240ca, 0x4971fa3, 0x81b17f8, 0x11ec34a5, 0x8366ddc, 0x1471809, 0xfa5f1c6, 0xc911e15,
	0x8849491, 0xcf4c2e2, 0x14471b91, 0x39f75be, 0x445c21e, 0xf1585e9, 0x72cc11f, 0x4c79f0c, 0xe5522e1,
	0x1874c1ee, 0x4444211, 0x7914884, 0x3d1b133, 0x25ba3c, 0x4194f65, 0x1c0457ef, 0xac4899d, 0xe1fa66c,
	0x130a7918, 0x9b8d312, 0x4b1c5c8, 0x61ccac3, 0x18c8aa6f, 0xe93cb0a, 0xdccb12c, 0xde10825, 0x969737d,
	0xf58c0c3, 0x7cee6a9, 0xc2c329a, 0xc7f9ed9, 0x107b3981, 0x696a40e, 0x152847ff, 0x4d88754, 0xb141f47,
	0x5a16ffe, 0x3a7870a, 0x18667659, 0x3b72b03, 0xb1c9435, 0x9285394, 0xa00005a, 0x37506c, 0x2edc0bb,
	0x19afe392, 0xeb39cac, 0x177ef286, 0xdf87197, 0x19f844ed, 0x31fe8, 0x15f9bfd, 0x80dbec, 0x342e96e,
	0x497aced, 0xe88e909, 0x1f5fa9ba, 0x530a6ee, 0x1ef4e3f1, 0x69ffd12, 0x583006d, 0x2ecc9b1, 0x362db70,
	0x18c7bdc5, 0xf4bb3c5, 0x1c90b957, 0xf067c09, 0x9768f2b, 0xf73566a, 0x1939a900, 0x198c38a, 0x202a2a1,
	0x4bbf5a6, 0x4e265bc, 0x1f44b6e7, 0x185ca49, 0xa39e81b, 0x24aff5b, 0x4acc9c2, 0x638bdd3, 0xb65b2a8,
	0x6def8be, 0xb94537a, 0x10b81dee, 0xe00ec55, 0x2f2cdf7, 0xc20622d, 0x2d20f36, 0xe03c8c9, 0x898ea76,
	0x8e3921b, 0x8905bff, 0x1e94b6c8, 0xee7ad86, 0x154797f2, 0xa620863, 0x3fbd0d9, 0x1f3caab, 0x30c24bd,
	0x19d3892f, 0x59c17a2, 0x1ab4b0ae, 0xf8714ee, 0x90c4098, 0xa9c800d, 0x1910236b, 0xea808d3, 0x9ae2f31,
	0x1a15ad64, 0xa48c8d1, 0x184635a4, 0xb725ef1, 0x11921dcc, 0x3f866df, 0x16c27568, 0xbdf580a, 0xb08f55c,
	0x186ee1c, 0xb1627fa, 0x34e82f6, 0x933837e, 0xf311be5, 0xfedb03b, 0x167f72cd, 0xa5469c0, 0x9c82531,
	0xb92a24b, 0x14fdc8b, 0x141980d1, 0xbdc3a49, 0x7e02bb1, 0xaf4e6dd, 0x106d99e1, 0xd4616fc, 0x93c2717,
	0x1c0a0507, 0xc6d5fed, 0x9a03d8b, 0xa1d22b0, 0x127853e3, 0xc4ac6b8, 0x1a048cf7, 0x9afb72c, 0x65d485d,
	0x72d5998, 0xe9fa744, 0xe49e82c, 0x253cf80, 0x5f777ce, 0xa3799a5, 0x17270cbb, 0xc1d1ef0, 0xdf74977,
	0x114cb859, 0xfa8e037, 0xb8f3fe5, 0xc734cc6, 0x70d3d61, 0xeadac62, 0x12093dd0, 0x9add67d, 0x87200d6,
	0x175bcbb, 0xb29b49f, 0x1806b79c, 0x12fb61f, 0x170b3a10, 0x3aaf1cf, 0xa224085, 0x79d26af, 0x97759e2,
	0x92e19f1, 0xb32714d, 0x1f00d9f1, 0xc728619, 0x9e6f627, 0xe745e24, 0x18ea4ace, 0xfc60a41, 0x125f5b2,
	0xc3cf512, 0x39ed486, 0xf4d15fa, 0xf9167fd, 0x1c1f5dd5, 0xc21a53e, 0x1897930, 0x957a112, 0x21059a0,
	0x1f9e3ddc, 0xa4dfced, 0x8427f6f, 0x726fbe7, 0x1ea658f8, 0x2fdcd4c, 0x17e9b66f, 0xb2e7c2e, 0x39923bf,
	0x1bae104, 0x3973ce5, 0xc6f264c, 0x3511b84, 0x124195d7, 0x11996bd, 0x20be23d, 0xdc437c4, 0x4b4f16b,
	0x11902a0, 0x6c29cc9, 0x1d5ffbe6, 0xdb0b4c7, 0x10144c14, 0x2f2b719, 0x301189, 0x2343336, 0xa0bf2ac,
}

// 大尾端转换小尾端
func sm2P256GetScalar(b *[32]byte, a []byte) {
	var scalarBytes []byte

	n := new(big.Int).SetBytes(a)
	if n.Cmp(sm2P256.N) >= 0 {
		n.Mod(n, sm2P256.N)
		scalarBytes = n.Bytes()
	} else {
		scalarBytes = a
	}
	for i, v := range scalarBytes {
		b[len(scalarBytes)-(1+i)] = v
	}
}

/**
 * (xOut, yOut, zOut) = (x1, y1, z1) + (x2, y2)
 *
 * let tx2 = x2 * z1 ^ 2, ty2 = y2 * z1 ^ 3
 * Then:
 *
 * 	xOut = (tx2 - x1)^3 - 2 * x1 * (tx2 - x1)^2
 *		yOut = (ty2 - t1)(x1 * (tx2 - x1)^2 - xOut) - y1 * (tx2 - x1)^3
 *		zOut = (tx2 - x1) * z1
**/
func sm2P256PointAddMixed(xOut, yOut, zOut, x1, y1, z1, x2, y2 *sm2P256FieldElement) {
	var z1z1, z1z1z1, ty2, tx2, dx, dx2, dx3, dy, dy2, v, tmp sm2P256FieldElement

	sm2P256Square(&z1z1, z1)
	sm2P256Mul2Way(&tx2, x2, &z1z1, &z1z1z1, z1, &z1z1)
	sm2P256Sub(&dx, &tx2, x1)
	sm2P256Mul2Way(zOut, z1, &dx, &ty2, y2, &z1z1z1)
	sm2P256Sub(&dy, &ty2, y1)
	sm2P256Square2Way(&dx2, &dx, &dy2, &dy)
	sm2P256Mul2Way(&dx3, &dx, &dx2, &v, x1, &dx2)
	sm2P256Sub(xOut, &dy2, &dx3)
	sm2P256Sub(xOut, xOut, &v)
	sm2P256Sub(xOut, xOut, &v)
	sm2P256Sub(&tmp, &v, xOut)
	sm2P256Mul2Way(yOut, &tmp, &dy, &tmp, y1, &dx3)
	sm2P256Sub(yOut, yOut, &tmp)
}

// sm2P256CopyConditional sets out=in if mask = 0xffffffff in constant time.
//
// On entry: mask is either 0 or 0xffffffff.
func sm2P256CopyConditional(out, in *sm2P256FieldElement, mask uint32) {
	for i := 0; i < 9; i++ {
		tmp := mask & (in[i] ^ out[i])
		out[i] ^= tmp
	}
}

// sm2P256SelectAffinePoint sets {out_x,out_y} to the index'th entry of table.
//
// On entry: index < 16, table[0] must be zero.
func sm2P256SelectAffinePoint(xOut, yOut *sm2P256FieldElement, table []uint32, index uint32) {
	xbase := index * 18
	ybase := xbase + 9
	for j := range xOut {
		xOut[j] = table[xbase+uint32(j)]
		yOut[j] = table[ybase+uint32(j)]
	}
}

// sm2P256SelectJacobianPoint sets {out_x,out_y,out_z} to the index'th entry of table.
//
// On entry: index < 16, table[0] must be zero.
func sm2P256SelectJacobianPoint(xOut, yOut, zOut *sm2P256FieldElement, table *[16][3]sm2P256FieldElement, index uint32) {
	for j := range xOut {
		xOut[j] = table[index][0][j]
		yOut[j] = table[index][1][j]
		zOut[j] = table[index][2][j]
	}
}

// sm2P256GetBit returns the bit'th bit of scalar.
func sm2P256GetBit(scalar *[32]uint8, bit uint) uint32 {
	return uint32(((scalar[bit>>3]) >> (bit & 7)) & 1)
}

// sm2P256ScalarBaseMult sets {xOut,yOut,zOut} = scalar*G where scalar is a
// little-endian number. Note that the value of scalar must be less than the
// order of the group.
func sm2P256ScalarBaseMult(xOut, yOut, zOut *sm2P256FieldElement, scalar *[32]uint8) {
	nIsInfinityMask := ^uint32(0)
	var px, py, tx, ty, tz sm2P256FieldElement
	var pIsNoninfiniteMask, mask, tableOffset uint32

	for i := range xOut {
		xOut[i] = 0
	}
	for i := range yOut {
		yOut[i] = 0
	}
	for i := range zOut {
		zOut[i] = 0
	}

	// The loop adds bits at positions 0, 64, 128 and 192, followed by
	// positions 32,96,160 and 224 and does this 32 times.
	for i := uint(0); i < 32; i++ {
		if i != 0 {
			sm2P256PointDouble(xOut, yOut, zOut, xOut, yOut, zOut)
		}
		tableOffset = 0
		for j := uint(0); j <= 32; j += 32 {
			bit0 := sm2P256GetBit(scalar, 31-i+j)
			bit1 := sm2P256GetBit(scalar, 95-i+j)
			bit2 := sm2P256GetBit(scalar, 159-i+j)
			bit3 := sm2P256GetBit(scalar, 223-i+j)
			index := bit0 | (bit1 << 1) | (bit2 << 2) | (bit3 << 3)

			sm2P256SelectAffinePoint(&px, &py, sm2P256Precomputed[tableOffset:], index)
			tableOffset += 30 * 9

			// Since scalar is less than the order of the group, we know that
			// {xOut,yOut,zOut} != {px,py,1}, unless both are zero, which we handle
			// below.
			sm2P256PointAddMixed(&tx, &ty, &tz, xOut, yOut, zOut, &px, &py)
			// The result of pointAddMixed is incorrect if {xOut,yOut,zOut} is zero
			// (a.k.a.  the point at infinity). We handle that situation by
			// copying the point from the table.
			sm2P256CopyConditional(xOut, &px, nIsInfinityMask)
			sm2P256CopyConditional(yOut, &py, nIsInfinityMask)
			sm2P256CopyConditional(zOut, &sm2P256Factor[1], nIsInfinityMask)

			// Equally, the result is also wrong if the point from the table is
			// zero, which happens when the index is zero. We handle that by
			// only copying from {tx,ty,tz} to {xOut,yOut,zOut} if index != 0.
			pIsNoninfiniteMask = poisitiveToAllOnes(index)
			mask = pIsNoninfiniteMask & ^nIsInfinityMask
			sm2P256CopyConditional(xOut, &tx, mask)
			sm2P256CopyConditional(yOut, &ty, mask)
			sm2P256CopyConditional(zOut, &tz, mask)
			// If p was not zero, then n is now non-zero.
			nIsInfinityMask &^= pIsNoninfiniteMask
		}
	}
}

// sm2P256ScalarBaseMult sets {xOut,yOut,zOut} = scalar*(x,y) where scalar is a
// little-endian number.
func sm2P256ScalarMult(xOut, yOut, zOut, x, y *sm2P256FieldElement, scalar *[32]uint8) {
	var precomp [16][3]sm2P256FieldElement
	var px, py, pz, tx, ty, tz sm2P256FieldElement
	var tIsInfinityMask, index, pIsNoninfiniteMask, mask uint32

	// We precompute 0,1,2,... times {x,y}.
	precomp[1][0] = *x
	precomp[1][1] = *y
	precomp[1][2] = sm2P256Factor[1]

	for i := 2; i < 16; i += 2 {
		half_i := i / 2
		i_plus_1 := i + 1
		sm2P256PointDouble(&precomp[i][0], &precomp[i][1], &precomp[i][2], &precomp[half_i][0], &precomp[half_i][1], &precomp[half_i][2])
		sm2P256PointAddMixed(&precomp[i_plus_1][0], &precomp[i_plus_1][1], &precomp[i_plus_1][2], &precomp[i][0], &precomp[i][1], &precomp[i][2], x, y)
	}

	// tIsInfinityMask = ^uint32(0)

	// We add in a window of four bits each iteration and do this 64 times.

	for i := 0; i < 64; i++ {
		if i != 0 {
			sm2P256PointDouble(xOut, yOut, zOut, xOut, yOut, zOut)
			sm2P256PointDouble(xOut, yOut, zOut, xOut, yOut, zOut)
			sm2P256PointDouble(xOut, yOut, zOut, xOut, yOut, zOut)
			sm2P256PointDouble(xOut, yOut, zOut, xOut, yOut, zOut)
		}

		index = uint32(scalar[31-i/2])
		if (i & 1) == 1 {
			index &= 15
		} else {
			index >>= 4
		}

		// See the comments in scalarBaseMult about handling infinities.
		sm2P256SelectJacobianPoint(&px, &py, &pz, &precomp, index)
		sm2P256PointAdd(xOut, yOut, zOut, &px, &py, &pz, &tx, &ty, &tz)
		sm2P256CopyConditional(xOut, &px, tIsInfinityMask)
		sm2P256CopyConditional(yOut, &py, tIsInfinityMask)
		sm2P256CopyConditional(zOut, &pz, tIsInfinityMask)

		pIsNoninfiniteMask = poisitiveToAllOnes(index)
		mask = pIsNoninfiniteMask & ^tIsInfinityMask
		sm2P256CopyConditional(xOut, &tx, mask)
		sm2P256CopyConditional(yOut, &ty, mask)
		sm2P256CopyConditional(zOut, &tz, mask)
		tIsInfinityMask &^= pIsNoninfiniteMask
	}
}

/**
 * xOut = x / z^2
 * yOut = y / z^3
**/
func sm2P256PointToAffine(xOut, yOut, x, y, z *sm2P256FieldElement) {
	var zInv, zInvSq sm2P256FieldElement

	zz := sm2P256ToBig(z)
	zz.ModInverse(zz, sm2P256.P)
	sm2P256FromBig(&zInv, zz)

	sm2P256Square(&zInvSq, &zInv)
	sm2P256Mul2Way(xOut, x, &zInvSq, &zInv, &zInv, &zInvSq)
	sm2P256Mul(yOut, y, &zInv)
}

func sm2P256ToAffine(x, y, z *sm2P256FieldElement) (xOut, yOut *big.Int) {
	var xx, yy sm2P256FieldElement

	sm2P256PointToAffine(&xx, &yy, x, y, z)
	return sm2P256ToBig(&xx), sm2P256ToBig(&yy)
}

var sm2P256Factor = []sm2P256FieldElement{
	{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
	{0x2, 0x0, 0x1FFFFF00, 0x7FF, 0x0, 0x0, 0x0, 0x2000000, 0x0},
	{0x4, 0x0, 0x1FFFFE00, 0xFFF, 0x0, 0x0, 0x0, 0x4000000, 0x0},
	{0x6, 0x0, 0x1FFFFD00, 0x17FF, 0x0, 0x0, 0x0, 0x6000000, 0x0},
	{0x8, 0x0, 0x1FFFFC00, 0x1FFF, 0x0, 0x0, 0x0, 0x8000000, 0x0},
	{0xA, 0x0, 0x1FFFFB00, 0x27FF, 0x0, 0x0, 0x0, 0xA000000, 0x0},
	{0xC, 0x0, 0x1FFFFA00, 0x2FFF, 0x0, 0x0, 0x0, 0xC000000, 0x0},
	{0xE, 0x0, 0x1FFFF900, 0x37FF, 0x0, 0x0, 0x0, 0xE000000, 0x0},
	{0x10, 0x0, 0x1FFFF800, 0x3FFF, 0x0, 0x0, 0x0, 0x0, 0x01},
}

// (x3, y3, z3) = (x1, y1, z1) + (x2, y2, z2)
func sm2P256PointAdd(x1, y1, z1, x2, y2, z2, x3, y3, z3 *sm2P256FieldElement) {

	var pv = []uint32{0x1ffffffe, 0xfffffff, 0x200000ff, 0xffff7ff, 0x1fffffff, 0xfffffff, 0x1fffffff, 0xdffffff, 0x1fffffff}
	var tx1, tx2, z22, z12, z23, z13, ty1, ty2, dx, dx2, dx3, dy, dy2, tm sm2P256FieldElement

	is_z1_zero := true
	for i := range z1 {
		if z1[i] != 0 {
			is_z1_zero = false
			break
		}
	}

	is_z1_p := true
	for i := range z1 {
		if z1[i] != pv[i] {
			is_z1_p = false
			break
		}
	}

	if is_z1_zero || is_z1_p {
		sm2P256Dup(x3, x2)
		sm2P256Dup(y3, y2)
		sm2P256Dup(z3, z2)
		return
	}

	is_z2_zero := true
	for i := range z2 {
		if z2[i] != 0 {
			is_z2_zero = false
			break
		}
	}

	is_z2_p := true
	for i := range z2 {
		if z2[i] != pv[i] {
			is_z2_p = false
			break
		}
	}

	if is_z2_zero || is_z2_p {
		sm2P256Dup(x3, x1)
		sm2P256Dup(y3, y1)
		sm2P256Dup(z3, z1)
		return
	}

	sm2P256Square2Way(&z12, z1, &z22, z2)
	sm2P256Mul2Way(&z13, &z12, z1, &z23, &z22, z2)
	sm2P256Mul2Way(&tx1, x1, &z22, &tx2, x2, &z12)
	sm2P256Mul2Way(&ty1, y1, &z23, &ty2, y2, &z13)

	// NOTE: remove this block does not affect the logic for now, but if it can be ignored
	// remains uncertain
	// if sm2P256ToBig(&tx1).Cmp(sm2P256ToBig(&tx2)) == 0 &&
	// 	sm2P256ToBig(&ty1).Cmp(sm2P256ToBig(&ty2)) == 0 {
	// 	fmt.Println("tx1 == tx2")
	// 	sm2P256PointDouble(x1, y1, z1, x1, y1, z1)
	// }

	// parallel 2
	sm2P256Sub(&dx, &tx2, &tx1) // dx = tx2 - tx1
	sm2P256Sub(&dy, &ty2, &ty1) // dy = ty2 - ty1

	sm2P256Square2Way(&dy2, &dy, &dx2, &dx)
	sm2P256Mul2Way(&dx3, &dx2, &dx, &tm, &tx1, &dx2)

	sm2P256Sub(x3, &dy2, &dx3)
	sm2P256Sub(x3, x3, &tm)  // x3 = dy ^ 2 - dx ^ 3 - tx1 * dx ^ 2
	sm2P256Sub(x3, x3, &tm)  // x3 = dy ^ 2 - dx ^ 3 - tx1 * dx ^ 2
	sm2P256Sub(&tm, &tm, x3) // tm = tx1 * dx ^ 2 - x3

	sm2P256Mul2Way(y3, &dy, &tm, z3, z1, z2)
	sm2P256Mul2Way(&tm, &dx3, &ty1, z3, z3, &dx)

	sm2P256Sub(y3, y3, &tm) // y3 = dy * (tx1 * dx ^ 2 - x3) - ty1 * dx ^ 3
}

func sm2P256PointDouble(x3, y3, z3, x, y, z *sm2P256FieldElement) {
	var x2, lambda, lambda2, z4_mul_a, y2, y4_mul_8, t, s sm2P256FieldElement

	sm2P256Square2Way(&x2, x, &z4_mul_a, z)
	sm2P256Square2Way(&y2, y, &z4_mul_a, &z4_mul_a)
	sm2P256Mul2Way(&z4_mul_a, &z4_mul_a, &sm2P256.a, &s, x, &y2) // s = x * y2

	sm2P256Add(&lambda, &x2, &x2)
	sm2P256Add(&lambda, &lambda, &x2)
	sm2P256Add(&lambda, &lambda, &z4_mul_a) // lambda = (3 * x2 + a * z4)

	sm2P256Add(&y4_mul_8, &y2, &y2)
	sm2P256Square2Way(&lambda2, &lambda, &y4_mul_8, &y4_mul_8)
	sm2P256Add(&y4_mul_8, &y4_mul_8, &y4_mul_8)
	sm2P256Add(&s, &s, &s)
	sm2P256Add(&s, &s, &s)       // s = 4x * y2
	sm2P256Sub(x3, &lambda2, &s) // x3 = 9 * x4 - 4 * x * y2
	sm2P256Sub(x3, x3, &s)       // x3 = 9 * x4 - 8 * x * y2

	sm2P256Sub(&t, &s, x3)                    // t = 4 * x * y2 - x3
	sm2P256Mul2Way(&t, &t, &lambda, z3, y, z) // t = (4 * x * y2 - x3) * 3 * x2

	sm2P256Sub(y3, &t, &y4_mul_8) // 8 * y4 - 3 * x2 * (s - x3)
	sm2P256Add(z3, z3, z3)
}

// p256Zero31 is 0 mod p.
var sm2P256Zero31 = sm2P256FieldElement{0x7FFFFFF8, 0x3FFFFFFC, 0x800003FC, 0x3FFFDFFC, 0x7FFFFFFC, 0x3FFFFFFC, 0x7FFFFFFC, 0x37FFFFFC, 0x7FFFFFFC}

func sm2P256Add(c, a, b *sm2P256FieldElement) {
	carry := uint32(0)
	c[0] = a[0] + b[0]
	c[0] += carry
	carry = c[0] >> 29
	c[0] &= bottom29BitsMask

	c[1] = a[1] + b[1]
	c[1] += carry
	carry = c[1] >> 28
	c[1] &= bottom28BitsMask

	c[2] = a[2] + b[2]
	c[2] += carry
	carry = c[2] >> 29
	c[2] &= bottom29BitsMask

	c[3] = a[3] + b[3]
	c[3] += carry
	carry = c[3] >> 28
	c[3] &= bottom28BitsMask

	c[4] = a[4] + b[4]
	c[4] += carry
	carry = c[4] >> 29
	c[4] &= bottom29BitsMask

	c[5] = a[5] + b[5]
	c[5] += carry
	carry = c[5] >> 28
	c[5] &= bottom28BitsMask

	c[6] = a[6] + b[6]
	c[6] += carry
	carry = c[6] >> 29
	c[6] &= bottom29BitsMask

	c[7] = a[7] + b[7]
	c[7] += carry
	carry = c[7] >> 28
	c[7] &= bottom28BitsMask

	c[8] = a[8] + b[8]
	c[8] += carry
	carry = c[8] >> 29
	c[8] &= bottom29BitsMask
	sm2P256ReduceCarry(c, carry)
}

// c = a - b
func sm2P256Sub(c, a, b *sm2P256FieldElement) {
	var carry uint32

	c[0] = a[0] - b[0]
	c[0] += sm2P256Zero31[0]
	c[0] += carry
	carry = c[0] >> 29
	c[0] &= bottom29BitsMask

	c[1] = a[1] - b[1]
	c[1] += sm2P256Zero31[1]
	c[1] += carry
	carry = c[1] >> 28
	c[1] &= bottom28BitsMask

	c[2] = a[2] - b[2]
	c[2] += sm2P256Zero31[2]
	c[2] += carry
	carry = c[2] >> 29
	c[2] &= bottom29BitsMask

	c[3] = a[3] - b[3]
	c[3] += sm2P256Zero31[3]
	c[3] += carry
	carry = c[3] >> 28
	c[3] &= bottom28BitsMask

	c[4] = a[4] - b[4]
	c[4] += sm2P256Zero31[4]
	c[4] += carry
	carry = c[4] >> 29
	c[4] &= bottom29BitsMask

	c[5] = a[5] - b[5]
	c[5] += sm2P256Zero31[5]
	c[5] += carry
	carry = c[5] >> 28
	c[5] &= bottom28BitsMask

	c[6] = a[6] - b[6]
	c[6] += sm2P256Zero31[6]
	c[6] += carry
	carry = c[6] >> 29
	c[6] &= bottom29BitsMask

	c[7] = a[7] - b[7]
	c[7] += sm2P256Zero31[7]
	c[7] += carry
	carry = c[7] >> 28
	c[7] &= bottom28BitsMask

	c[8] = a[8] - b[8]
	c[8] += sm2P256Zero31[8]
	c[8] += carry
	carry = c[8] >> 29
	c[8] &= bottom29BitsMask

	sm2P256ReduceCarry(c, carry)
}

func sm2P256Mul(c, a, b *sm2P256FieldElement) {
	var tmp sm2P256LargeFieldElement

	tmp[0] = uint64(a[0]) * uint64(b[0])

	tmp[1] = uint64(a[0]) * uint64(b[1])
	tmp[1] += uint64(a[1]) * uint64(b[0])

	tmp[2] = uint64(a[0]) * uint64(b[2])
	tmp[2] += uint64(a[1]) * (uint64(b[1]) << 1)
	tmp[2] += uint64(a[2]) * uint64(b[0])

	tmp[3] = uint64(a[0]) * uint64(b[3])
	tmp[3] += uint64(a[1]) * uint64(b[2])
	tmp[3] += uint64(a[2]) * uint64(b[1])
	tmp[3] += uint64(a[3]) * uint64(b[0])

	tmp[4] = uint64(a[1]) * uint64(b[3])
	tmp[4] += uint64(a[3]) * uint64(b[1])
	tmp[4] <<= 1
	tmp[4] += uint64(a[0]) * uint64(b[4])
	tmp[4] += uint64(a[2]) * uint64(b[2])
	tmp[4] += uint64(a[4]) * uint64(b[0])

	tmp[5] = uint64(a[0]) * uint64(b[5])
	tmp[5] += uint64(a[1]) * uint64(b[4])
	tmp[5] += uint64(a[2]) * uint64(b[3])
	tmp[5] += uint64(a[3]) * uint64(b[2])
	tmp[5] += uint64(a[4]) * uint64(b[1])
	tmp[5] += uint64(a[5]) * uint64(b[0])

	tmp[6] = uint64(a[1]) * uint64(b[5])
	tmp[6] += uint64(a[3]) * uint64(b[3])
	tmp[6] += uint64(a[5]) * uint64(b[1])
	tmp[6] <<= 1
	tmp[6] += uint64(a[0]) * uint64(b[6])
	tmp[6] += uint64(a[2]) * uint64(b[4])
	tmp[6] += uint64(a[4]) * uint64(b[2])
	tmp[6] += uint64(a[6]) * uint64(b[0])

	tmp[7] = uint64(a[0]) * uint64(b[7])
	tmp[7] += uint64(a[1]) * uint64(b[6])
	tmp[7] += uint64(a[2]) * uint64(b[5])
	tmp[7] += uint64(a[3]) * uint64(b[4])
	tmp[7] += uint64(a[4]) * uint64(b[3])
	tmp[7] += uint64(a[5]) * uint64(b[2])
	tmp[7] += uint64(a[6]) * uint64(b[1])
	tmp[7] += uint64(a[7]) * uint64(b[0])

	tmp[8] = uint64(a[1]) * uint64(b[7])
	tmp[8] += uint64(a[3]) * uint64(b[5])
	tmp[8] += uint64(a[5]) * uint64(b[3])
	tmp[8] += uint64(a[7]) * uint64(b[1])
	tmp[8] <<= 1
	tmp[8] += uint64(a[0]) * uint64(b[8])
	tmp[8] += uint64(a[2]) * uint64(b[6])
	tmp[8] += uint64(a[4]) * uint64(b[4])
	tmp[8] += uint64(a[6]) * uint64(b[2])
	tmp[8] += uint64(a[8]) * uint64(b[0])

	tmp[9] = uint64(a[1]) * uint64(b[8])
	tmp[9] += uint64(a[2]) * uint64(b[7])
	tmp[9] += uint64(a[3]) * uint64(b[6])
	tmp[9] += uint64(a[4]) * uint64(b[5])
	tmp[9] += uint64(a[5]) * uint64(b[4])
	tmp[9] += uint64(a[6]) * uint64(b[3])
	tmp[9] += uint64(a[7]) * uint64(b[2])
	tmp[9] += uint64(a[8]) * uint64(b[1])

	tmp[10] = uint64(a[3]) * uint64(b[7])
	tmp[10] += uint64(a[5]) * uint64(b[5])
	tmp[10] += uint64(a[7]) * uint64(b[3])
	tmp[10] <<= 1
	tmp[10] += uint64(a[2]) * uint64(b[8])
	tmp[10] += uint64(a[4]) * uint64(b[6])
	tmp[10] += uint64(a[6]) * uint64(b[4])
	tmp[10] += uint64(a[8]) * uint64(b[2])

	tmp[11] = uint64(a[3]) * uint64(b[8])
	tmp[11] += uint64(a[4]) * uint64(b[7])
	tmp[11] += uint64(a[5]) * uint64(b[6])
	tmp[11] += uint64(a[6]) * uint64(b[5])
	tmp[11] += uint64(a[7]) * uint64(b[4])
	tmp[11] += uint64(a[8]) * uint64(b[3])

	tmp[12] = uint64(a[5]) * uint64(b[7])
	tmp[12] += uint64(a[7]) * uint64(b[5])
	tmp[12] <<= 1
	tmp[12] += uint64(a[4]) * uint64(b[8])
	tmp[12] += uint64(a[6]) * uint64(b[6])
	tmp[12] += uint64(a[8]) * uint64(b[4])

	tmp[13] = uint64(a[5]) * uint64(b[8])
	tmp[13] += uint64(a[6]) * uint64(b[7])
	tmp[13] += uint64(a[7]) * uint64(b[6])
	tmp[13] += uint64(a[8]) * uint64(b[5])

	tmp[14] = uint64(a[6]) * uint64(b[8])
	tmp[14] += uint64(a[7]) * uint64(b[7]) << 1
	tmp[14] += uint64(a[8]) * uint64(b[6])

	tmp[15] = uint64(a[7]) * uint64(b[8])
	tmp[15] += uint64(a[8]) * uint64(b[7])

	tmp[16] = uint64(a[8]) * uint64(b[8])

	sm2P256ReduceDegree(c, &tmp)
}

func sm2P256Mul2Way(c, a1, b1, c2, a2, b2 *sm2P256FieldElement) {
	var tmp1, tmp2 sm2P256LargeFieldElement

	addr_a1 := &a1[0]
	addrA1 := uintptr(unsafe.Pointer(addr_a1))
	addr_b1 := &b1[0]
	addrB1 := uintptr(unsafe.Pointer(addr_b1))
	addr_a2 := &a2[0]
	addrA2 := uintptr(unsafe.Pointer(addr_a2))
	addr_b2 := &b2[0]
	addrB2 := uintptr(unsafe.Pointer(addr_b2))
	addr_tmp1 := &tmp1[0]
	addrTmp1 := uintptr(unsafe.Pointer(addr_tmp1))
	addr_tmp2 := &tmp2[0]
	addrTmp2 := uintptr(unsafe.Pointer(addr_tmp2))
	_sm2P256Mul2Way1((*uint64)(unsafe.Pointer(addrTmp1)), (*uint32)(unsafe.Pointer(addrA1)),
		(*uint32)(unsafe.Pointer(addrB1)), (*uint64)(unsafe.Pointer(addrTmp2)),
		(*uint32)(unsafe.Pointer(addrA2)), (*uint32)(unsafe.Pointer(addrB2)))

	tmp1[8] = uint64(a1[1]) * uint64(b1[7])
	tmp1[8] += uint64(a1[3]) * uint64(b1[5])
	tmp1[8] += uint64(a1[5]) * uint64(b1[3])
	tmp1[8] += uint64(a1[7]) * uint64(b1[1])
	tmp1[8] <<= 1
	tmp1[8] += uint64(a1[0]) * uint64(b1[8])
	tmp1[8] += uint64(a1[2]) * uint64(b1[6])
	tmp1[8] += uint64(a1[4]) * uint64(b1[4])
	tmp1[8] += uint64(a1[6]) * uint64(b1[2])
	tmp1[8] += uint64(a1[8]) * uint64(b1[0])

	tmp2[8] = uint64(a2[1]) * uint64(b2[7])
	tmp2[8] += uint64(a2[3]) * uint64(b2[5])
	tmp2[8] += uint64(a2[5]) * uint64(b2[3])
	tmp2[8] += uint64(a2[7]) * uint64(b2[1])
	tmp2[8] <<= 1
	tmp2[8] += uint64(a2[0]) * uint64(b2[8])
	tmp2[8] += uint64(a2[2]) * uint64(b2[6])
	tmp2[8] += uint64(a2[4]) * uint64(b2[4])
	tmp2[8] += uint64(a2[6]) * uint64(b2[2])
	tmp2[8] += uint64(a2[8]) * uint64(b2[0])

	_sm2P256Mul2Way2((*uint64)(unsafe.Pointer(addrTmp1)), (*uint32)(unsafe.Pointer(addrA1)),
		(*uint32)(unsafe.Pointer(addrB1)), (*uint64)(unsafe.Pointer(addrTmp2)),
		(*uint32)(unsafe.Pointer(addrA2)), (*uint32)(unsafe.Pointer(addrB2)))

	sm2P256ReduceDegree2Way(c, c2, &tmp1, &tmp2)
	// sm2P256ReduceDegree(c, &tmp1)
	// sm2P256ReduceDegree(c2, &tmp2)
	// return tmp1, tmp2
}

func sm2P256Square(b, a *sm2P256FieldElement) {

	var tmp sm2P256LargeFieldElement

	tmp[0] = uint64(a[0]) * uint64(a[0])

	tmp[1] = uint64(a[0]) * uint64(a[1]) << 1

	tmp[2] = uint64(a[0]) * uint64(a[2])
	tmp[2] += uint64(a[1]) * uint64(a[1])
	tmp[2] <<= 1

	tmp[3] = uint64(a[0]) * uint64(a[3])
	tmp[3] += uint64(a[1]) * uint64(a[2])
	tmp[3] <<= 1

	tmp[4] = uint64(a[0]) * uint64(a[4])
	tmp[4] += uint64(a[1]) * uint64(a[3]) << 1
	tmp[4] <<= 1
	tmp[4] += uint64(a[2]) * uint64(a[2])

	tmp[5] = uint64(a[0]) * uint64(a[5])
	tmp[5] += uint64(a[1]) * uint64(a[4])
	tmp[5] += uint64(a[2]) * uint64(a[3])
	tmp[5] <<= 1

	tmp[6] = uint64(a[0]) * uint64(a[6])
	tmp[6] += uint64(a[1]) * uint64(a[5]) << 1
	tmp[6] += uint64(a[2]) * uint64(a[4])
	tmp[6] += uint64(a[3]) * uint64(a[3])
	tmp[6] <<= 1

	tmp[7] = uint64(a[0]) * uint64(a[7])
	tmp[7] += uint64(a[1]) * uint64(a[6])
	tmp[7] += uint64(a[2]) * uint64(a[5])
	tmp[7] += uint64(a[3]) * uint64(a[4])
	tmp[7] <<= 1

	tmp[8] = uint64(a[0]) * uint64(a[8])
	tmp[8] += uint64(a[1]) * uint64(a[7]) << 1
	tmp[8] += uint64(a[2]) * uint64(a[6])
	tmp[8] += uint64(a[3]) * uint64(a[5]) << 1
	tmp[8] <<= 1
	tmp[8] += uint64(a[4]) * uint64(a[4])

	tmp[9] = uint64(a[1]) * uint64(a[8])
	tmp[9] += uint64(a[2]) * uint64(a[7])
	tmp[9] += uint64(a[3]) * uint64(a[6])
	tmp[9] += uint64(a[4]) * uint64(a[5])
	tmp[9] <<= 1

	tmp[10] = uint64(a[2]) * uint64(a[8])
	tmp[10] += uint64(a[3]) * uint64(a[7]) << 1
	tmp[10] += uint64(a[4]) * uint64(a[6])
	tmp[10] += uint64(a[5]) * uint64(a[5])
	tmp[10] <<= 1

	tmp[11] = uint64(a[3]) * uint64(a[8])
	tmp[11] += uint64(a[4]) * uint64(a[7])
	tmp[11] += uint64(a[5]) * uint64(a[6])
	tmp[11] <<= 1

	tmp[12] = uint64(a[4]) * uint64(a[8])
	tmp[12] += uint64(a[5]) * uint64(a[7]) << 1
	tmp[12] <<= 1
	tmp[12] += uint64(a[6]) * uint64(a[6])

	tmp[13] = uint64(a[5]) * uint64(a[8])
	tmp[13] += uint64(a[6]) * uint64(a[7])
	tmp[13] <<= 1

	tmp[14] = uint64(a[6]) * uint64(a[8])
	tmp[14] += uint64(a[7]) * uint64(a[7])
	tmp[14] <<= 1

	tmp[15] = uint64(a[7]) * uint64(a[8]) << 1

	tmp[16] = uint64(a[8]) * uint64(a[8])

	sm2P256ReduceDegree(b, &tmp)
}

func sm2P256Square2Way(b, a, b2, a2 *sm2P256FieldElement) {
	var tmp, tmp2 sm2P256LargeFieldElement

	addr1 := &tmp[0]
	addrTMP1 := uintptr(unsafe.Pointer(addr1))
	addr1 = &tmp2[0]
	addrTMP2 := uintptr(unsafe.Pointer(addr1))

	addr2 := &a[0]
	addrA := uintptr(unsafe.Pointer(addr2))
	addr2 = &a2[0]
	addrA2 := uintptr(unsafe.Pointer(addr2))

	_sm2P256Square2Way((*uint64)(unsafe.Pointer(addrTMP1)), (*uint32)(unsafe.Pointer(addrA)),
		(*uint64)(unsafe.Pointer(addrTMP2)), (*uint32)(unsafe.Pointer(addrA2)))

	sm2P256ReduceDegree2Way(b, b2, &tmp, &tmp2)
}

// poisitiveToAllOnes returns:
//   0xffffffff for 0 < x <= 2**31
//   0 for x == 0 or x > 2**31.
func poisitiveToAllOnes(x uint32) uint32 {
	return ((x - 1) >> 31) - 1
}

var sm2P256Carry = [8 * 9]uint32{
	0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
	0x2, 0x0, 0x1FFFFF00, 0x7FF, 0x0, 0x0, 0x0, 0x2000000, 0x0,
	0x4, 0x0, 0x1FFFFE00, 0xFFF, 0x0, 0x0, 0x0, 0x4000000, 0x0,
	0x6, 0x0, 0x1FFFFD00, 0x17FF, 0x0, 0x0, 0x0, 0x6000000, 0x0,
	0x8, 0x0, 0x1FFFFC00, 0x1FFF, 0x0, 0x0, 0x0, 0x8000000, 0x0,
	0xA, 0x0, 0x1FFFFB00, 0x27FF, 0x0, 0x0, 0x0, 0xA000000, 0x0,
	0xC, 0x0, 0x1FFFFA00, 0x2FFF, 0x0, 0x0, 0x0, 0xC000000, 0x0,
	0xE, 0x0, 0x1FFFF900, 0x37FF, 0x0, 0x0, 0x0, 0xE000000, 0x0,
}

// carry < 2 ^ 3
// p的P256表示
// FFFFFFF EFFFFFF 1FFFFFFF FFFFFFF 1FFFFFFF FFFFC00 7F FFFFFFF 1FFFFFFFF
// -2p的P256表示
// 0 0 2000000 0 0 0 7ff 1fffff00 0 2
//
func sm2P256ReduceCarry(a *sm2P256FieldElement, carry uint32) {
	a[0] += sm2P256Carry[carry*9+0]
	a[2] += sm2P256Carry[carry*9+2]
	a[3] += sm2P256Carry[carry*9+3]
	a[7] += sm2P256Carry[carry*9+7]
}

// 计算 (b + (b*pprime mod r) * p) / r
func sm2P256ReduceDegree(a *sm2P256FieldElement, b *sm2P256LargeFieldElement) {
	var tmp64 [10]uint64
	var carry uint32
	var x64 uint64
	j, j1, j2, j3, j4, j5 := 0, 1, 2, 3, 4, 5

	sm2P256FromLargeElement(&tmp64, b)

	// 后一位超出来的部分加到前一位上去
	tmp64[j1] += tmp64[j] >> 57
	x64 = tmp64[j] & bottom57BitsMask
	if x64 > 0 {

		tmp64[j1] += (x64 << 7) & bottom57BitsMask
		tmp64[j2] += x64 >> 50

		tmp64[j1] += twoPower57
		tmp64[j2] += bottom57BitsMask

		tmp64[j1] -= (x64 << 39) & bottom57BitsMask
		tmp64[j2] -= x64 >> 18

		tmp64[j3] += bottom57BitsMask
		tmp64[j4] += bottom57BitsMask

		tmp64[j3] -= (x64 << 53) & bottom57BitsMask
		tmp64[j4] -= (x64 >> 4) & bottom57BitsMask

		tmp64[j5] -= 1
		tmp64[j4] += (x64 << 28) & bottom57BitsMask
		tmp64[j5] += (x64 >> 29) & bottom29BitsMask

	}
	j, j1, j2, j3, j4, j5 = j1, j2, j3, j4, j5, j5+1

	tmp64[j1] += tmp64[j] >> 57
	x64 = tmp64[j] & bottom57BitsMask
	if x64 > 0 {

		tmp64[j1] += (x64 << 7) & bottom57BitsMask
		tmp64[j2] += x64 >> 50

		tmp64[j1] += twoPower57
		tmp64[j2] += bottom57BitsMask

		tmp64[j1] -= (x64 << 39) & bottom57BitsMask
		tmp64[j2] -= x64 >> 18

		tmp64[j3] += bottom57BitsMask
		tmp64[j4] += bottom57BitsMask

		tmp64[j3] -= (x64 << 53) & bottom57BitsMask
		tmp64[j4] -= (x64 >> 4) & bottom57BitsMask

		tmp64[j5] -= 1
		tmp64[j4] += (x64 << 28) & bottom57BitsMask
		tmp64[j5] += (x64 >> 29) & bottom29BitsMask

	}
	j, j1, j2, j3, j4, j5 = j1, j2, j3, j4, j5, j5+1

	tmp64[j1] += tmp64[j] >> 57
	x64 = tmp64[j] & bottom57BitsMask
	if x64 > 0 {

		tmp64[j1] += (x64 << 7) & bottom57BitsMask
		tmp64[j2] += x64 >> 50

		tmp64[j1] += twoPower57
		tmp64[j2] += bottom57BitsMask

		tmp64[j1] -= (x64 << 39) & bottom57BitsMask
		tmp64[j2] -= x64 >> 18

		tmp64[j3] += bottom57BitsMask
		tmp64[j4] += bottom57BitsMask

		tmp64[j3] -= (x64 << 53) & bottom57BitsMask
		tmp64[j4] -= (x64 >> 4) & bottom57BitsMask

		tmp64[j5] -= 1
		tmp64[j4] += (x64 << 28) & bottom57BitsMask
		tmp64[j5] += (x64 >> 29) & bottom29BitsMask

	}
	j, j1, j2, j3, j4, j5 = j1, j2, j3, j4, j5, j5+1

	tmp64[j1] += tmp64[j] >> 57
	x64 = tmp64[j] & bottom57BitsMask
	if x64 > 0 {

		tmp64[j1] += (x64 << 7) & bottom57BitsMask
		tmp64[j2] += x64 >> 50

		tmp64[j1] += twoPower57
		tmp64[j2] += bottom57BitsMask

		tmp64[j1] -= (x64 << 39) & bottom57BitsMask
		tmp64[j2] -= x64 >> 18

		tmp64[j3] += bottom57BitsMask
		tmp64[j4] += bottom57BitsMask

		tmp64[j3] -= (x64 << 53) & bottom57BitsMask
		tmp64[j4] -= (x64 >> 4) & bottom57BitsMask

		tmp64[j5] -= 1
		tmp64[j4] += (x64 << 28) & bottom57BitsMask
		tmp64[j5] += (x64 >> 29) & bottom29BitsMask

	}
	j, j1, j2, j3, j4, j5 = j1, j2, j3, j4, j5, j5+1

	x64 = tmp64[j] & bottom29BitsMask
	tmp64[j] = (tmp64[j] >> 29) << 29

	if x64 > 0 {

		tmp64[j1] += (x64 << 7) & bottom57BitsMask
		tmp64[j2] += x64 >> 50

		tmp64[j1] += twoPower57
		tmp64[j2] += bottom57BitsMask

		tmp64[j1] -= (x64 << 39) & bottom57BitsMask
		tmp64[j2] -= x64 >> 18

		tmp64[j3] += bottom57BitsMask
		tmp64[j4] += bottom57BitsMask

		tmp64[j3] -= (x64 << 53) & bottom57BitsMask
		tmp64[j4] -= (x64 >> 4) & bottom57BitsMask

		tmp64[j5] -= 1
		tmp64[j4] += (x64 << 28) & bottom57BitsMask
		tmp64[j5] += (x64 >> 29) & bottom29BitsMask

	}

	if tmp64[9]+1 == 0 {
		tmp64[9] = 0
		tmp64[8] -= twoPower57
	}

	carry = sm2P256DivideByR(a, &tmp64)
	// fmt.Println(carry)
	sm2P256ReduceCarry(a, carry)
}

func sm2P256ReduceDegree2Way(a, a2 *sm2P256FieldElement, b, b2 *sm2P256LargeFieldElement) {
	var tmp64, tmp642 [10]uint64
	var carry, carry2 uint32

	addr1 := &tmp64[0]
	addrTMP1 := uintptr(unsafe.Pointer(addr1))
	addr1 = &tmp642[0]
	addrTMP2 := uintptr(unsafe.Pointer(addr1))

	addr1 = &b[0]
	addrB1 := uintptr(unsafe.Pointer(addr1))
	addr1 = &b2[0]
	addrB2 := uintptr(unsafe.Pointer(addr1))

	addr2 := &a[0]
	addrA := uintptr(unsafe.Pointer(addr2))
	addr2 = &a2[0]
	addrA2 := uintptr(unsafe.Pointer(addr2))

	// sm2P256FromLargeElement(&tmp64, b)
	// sm2P256FromLargeElement(&tmp642, b2)
	// _sm2P256FromLargeElement_2Way((*uint64)(unsafe.Pointer(addrTMP1)), (*uint64)(unsafe.Pointer(addrB1)),
	// 	(*uint64)(unsafe.Pointer(addrTMP2)), (*uint64)(unsafe.Pointer(addrB2)))

	// _reduceDegree_2wayNew((*uint64)(unsafe.Pointer(addrTMP1)), (*uint64)(unsafe.Pointer(addrTMP2)))

	// carry_temp := _sm2P256DivideByR_2way((*uint32)(unsafe.Pointer(addrA)), (*uint32)(unsafe.Pointer(addrA2)),
	// 	(*uint64)(unsafe.Pointer(addrTMP1)), (*uint64)(unsafe.Pointer(addrTMP2)))
	// carry = sm2P256DivideByR(a, &tmp64)
	// carry2 = sm2P256DivideByR(a2, &tmp642)

	carry_temp := _sm2ReduceDegree_2way((*uint32)(unsafe.Pointer(addrA)), (*uint32)(unsafe.Pointer(addrA2)),
		(*uint64)(unsafe.Pointer(addrB1)), (*uint64)(unsafe.Pointer(addrB2)),
		(*uint64)(unsafe.Pointer(addrTMP1)), (*uint64)(unsafe.Pointer(addrTMP2)))
	carry = uint32(carry_temp)
	carry2 = uint32(carry_temp >> 32)

	// fmt.Println(carry_temp, carry, carry2)
	sm2P256ReduceCarry(a, carry)
	sm2P256ReduceCarry(a2, carry2)
	// return carry, carry2
}

func sm2P256DivideByR(a *sm2P256FieldElement, tmp *[10]uint64) (carry uint32) {
	a[0] = uint32(tmp[4] >> 29)
	a[0] += uint32(tmp[5]<<28) & bottom29BitsMask
	carry = a[0] >> 29
	a[0] &= bottom29BitsMask

	a[1] = uint32(tmp[5]>>1) & bottom28BitsMask
	a[1] += carry
	carry = a[1] >> 28
	a[1] &= bottom28BitsMask

	a[2] = uint32(tmp[5] >> 29)
	a[2] += carry
	a[2] += uint32(tmp[6]<<28) & bottom29BitsMask
	carry = a[2] >> 29
	a[2] &= bottom29BitsMask

	a[3] = uint32(tmp[6]>>1) & bottom28BitsMask
	a[3] += carry
	carry = a[3] >> 28
	a[3] &= bottom28BitsMask

	a[4] = uint32(tmp[6] >> 29)
	a[4] += carry
	a[4] += uint32(tmp[7]<<28) & bottom29BitsMask
	carry = a[4] >> 29
	a[4] &= bottom29BitsMask

	a[5] = uint32(tmp[7]>>1) & bottom28BitsMask
	a[5] += carry
	carry = a[5] >> 28
	a[5] &= bottom28BitsMask

	a[6] = uint32(tmp[7] >> 29)
	a[6] += carry
	a[6] += uint32(tmp[8]<<28) & bottom29BitsMask
	carry = a[6] >> 29
	a[6] &= bottom29BitsMask

	a[7] = uint32(tmp[8]>>1) & bottom28BitsMask
	a[7] += carry
	carry = a[7] >> 28
	a[7] &= bottom28BitsMask

	a[8] = uint32(tmp[8] >> 29)
	a[8] += carry
	a[8] += (uint32(tmp[9] << 28)) & bottom29BitsMask
	carry = a[8] >> 29
	a[8] &= bottom29BitsMask
	return
}

func sm2P256FromLargeElement(a *[10]uint64, b *sm2P256LargeFieldElement) {
	var carry uint64

	a[0] = b[0]
	a[0] += ((b[1] << 29) & bottom57BitsMask)
	carry = a[0] >> 57
	a[0] = a[0] & bottom57BitsMask

	a[1] = carry
	a[1] += b[1] >> 28
	a[1] += b[2]
	a[1] += (b[3] << 29) & bottom57BitsMask
	carry = a[1] >> 57
	a[1] = a[1] & bottom57BitsMask

	a[2] = carry
	a[2] += b[3] >> 28
	a[2] += b[4]
	a[2] += (b[5] << 29) & bottom57BitsMask
	carry = a[2] >> 57
	a[2] = a[2] & bottom57BitsMask

	a[3] = carry
	a[3] += b[5] >> 28
	a[3] += b[6]
	a[3] += (b[7] << 29) & bottom57BitsMask
	carry = a[3] >> 57
	a[3] = a[3] & bottom57BitsMask

	a[4] = carry
	a[4] += b[7] >> 28
	a[4] += b[8]
	a[4] += (b[9] << 29) & bottom57BitsMask
	carry = a[4] >> 57
	a[4] = a[4] & bottom57BitsMask

	a[5] = carry
	a[5] += b[9] >> 28
	a[5] += b[10]
	a[5] += (b[11] << 29) & bottom57BitsMask
	carry = a[5] >> 57
	a[5] = a[5] & bottom57BitsMask

	a[6] = carry
	a[6] += b[11] >> 28
	a[6] += b[12]
	a[6] += (b[13] << 29) & bottom57BitsMask
	carry = a[6] >> 57
	a[6] = a[6] & bottom57BitsMask

	a[7] = carry
	a[7] += b[13] >> 28
	a[7] += b[14]
	a[7] += (b[15] << 29) & bottom57BitsMask
	carry = a[7] >> 57
	a[7] = a[7] & bottom57BitsMask

	a[8] = carry
	a[8] += b[15] >> 28
	a[8] += b[16]
	a[9] = 0
}

// b = a
func sm2P256Dup(b, a *sm2P256FieldElement) {
	*b = *a
}

func getBottomNBitsMask(n uint32) uint32 {
	if n == 28 {
		return bottom28BitsMask
	}
	if n == 29 {
		return bottom29BitsMask
	}
	return 0
}

func getBottomNBits(x *big.Int, n uint32) uint32 {
	if bits := x.Bits(); len(bits) > 0 {
		return uint32(bits[0]) & getBottomNBitsMask(n)
	} else {
		return 0
	}
}

func getBottom29Bits(x *big.Int) uint32 {
	return getBottomNBits(x, 29)
}

func getBottom28Bits(x *big.Int) uint32 {
	return getBottomNBits(x, 28)
}

// 把a表示成长度为29,28,...,28,29（共9个元素）的数组
func sm2P256FromBigPlain(X *sm2P256FieldElement, x *big.Int) {

	X[0] = getBottom29Bits(x)
	x.Rsh(x, 29)

	i := 1
	for i < 8 {
		X[i] = getBottom28Bits(x)
		x.Rsh(x, 28)
		i++

		X[i] = getBottom29Bits(x)
		x.Rsh(x, 29)
		i++
	}
}

// X = a * R mod P (R = 2**257)
func sm2P256FromBig(X *sm2P256FieldElement, a *big.Int) {
	x := new(big.Int).Lsh(a, 257)
	x.Mod(x, sm2P256.P)
	sm2P256FromBigPlain(X, x)
}

// X = r * R mod P
// r = X * R' mod P
func sm2P256ToBig(X *sm2P256FieldElement) *big.Int {
	r := sm2P256ToBigPlain(X)
	r.Mul(r, sm2P256.RInverse)
	r.Mod(r, sm2P256.P)
	return r
}

func sm2P256ToBigPlain(X *sm2P256FieldElement) *big.Int {
	r, tm := new(big.Int), new(big.Int)
	r.SetInt64(int64(X[8]))

	i := 7
	for i >= 0 {
		r.Lsh(r, 28)
		tm.SetInt64(int64(X[i]))
		r.Add(r, tm)
		i--
		r.Lsh(r, 29)
		tm.SetInt64(int64(X[i]))
		r.Add(r, tm)
		i--
	}
	return r
}
