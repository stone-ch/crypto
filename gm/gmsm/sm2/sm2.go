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

// reference to ecdsa
import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha512"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"io"
	"math/big"

	"github.com/xuperchain/crypto/gm/gmsm/sm3"
	//	"github.com/tjfoc/gmsm/sm3"
)

const (
	aesIV = "IV for <SM2> CTR"
)

type PublicKey struct {
	elliptic.Curve
	X, Y *big.Int
}

type PrivateKey struct {
	PublicKey
	D *big.Int
}

type Signer struct {
	PrivateKey
	Msg     []byte
	Entropy []byte
	AES_key []byte
	CSPRNG  cipher.StreamReader

	e, k, r, t, s *big.Int
}

type sm2Signature struct {
	R, S *big.Int
}

// Signer

func (signer *Signer) MakeEntropy() (err error) {
	entropylen := (signer.Curve.Params().BitSize + 7) / 16
	if entropylen > 32 {
		entropylen = 32
	}

	signer.Entropy = make([]byte, entropylen)
	_, err = io.ReadFull(rand.Reader, signer.Entropy)
	if err != nil {
		return err
	}

	return
}

func (signer *Signer) MakeAESKey() {
	md := sha512.New()
	md.Write(signer.D.Bytes())        // the private key,
	md.Write(signer.Entropy)          // the entropy,
	md.Write(signer.Msg)              // and the input hash;
	signer.AES_key = md.Sum(nil)[:32] // and compute ChopMD-256(SHA-512),
}

func (signer *Signer) MakeCSPRNG() (err error) {
	block, err := aes.NewCipher(signer.AES_key)
	if err != nil {
		return err
	}

	signer.CSPRNG = cipher.StreamReader{
		R: zeroReader,
		S: cipher.NewCTR(block, []byte(aesIV)),
	}

	return
}

func (signer *Signer) IsParamsValid() (valid bool) {
	return signer.Curve.Params().N.Sign() != 0
}

// make e, k, r, t
func (signer *Signer) MakeEKRT() (err error) {
	signer.e = new(big.Int).SetBytes(signer.Msg)
	for {
		signer.k, err = randFieldElement(signer.Curve, signer.CSPRNG)
		if err != nil {
			return
		}

		signer.r, _ = signer.Curve.ScalarBaseMult(signer.k.Bytes())
		signer.r.Add(signer.r, signer.e)
		signer.r.Mod(signer.r, signer.Params().N)
		// TODO: the conditions here should be checked
		if signer.r.Sign() != 0 {
			return
		}

		signer.t = new(big.Int).Add(signer.r, signer.t)
		if signer.t.Cmp(signer.Params().N) == 0 {
			return
		}

	}
}

func (signer *Signer) MakeSignature() (err error) {
	for {
		err = signer.MakeEKRT()
		if err != nil {
			return
		}

		rD := new(big.Int).Mul(signer.D, signer.r)
		signer.s = new(big.Int).Sub(signer.k, rD)
		d1 := new(big.Int).Add(signer.D, one)
		d1Inv := new(big.Int).ModInverse(d1, signer.Params().N)
		signer.s.Mul(signer.s, d1Inv)
		signer.s.Mod(signer.s, signer.Params().N)
		if signer.s.Sign() != 0 {
			return
		}
	}
}

// sign format = 30 + len(z) + 02 + len(r) + r + 02 + len(s) + s, z being what follows its size, ie 02+len(r)+r+02+len(s)+s
func (signer *Signer) Sign() ([]byte, error) {

	err := signer.MakeEntropy()
	if err != nil {
		return nil, err
	}

	signer.MakeAESKey()

	err = signer.MakeCSPRNG()
	if err != nil {
		return nil, err
	}

	if !signer.IsParamsValid() {
		return nil, errors.New("the order of the group is zero")
	}

	err = signer.MakeSignature()
	if err != nil {
		return nil, err
	}

	return asn1.Marshal(sm2Signature{signer.r, signer.s})
}

// PrivateKey

func (priv *PrivateKey) Public() crypto.PublicKey {
	return &priv.PublicKey
}

func (priv *PrivateKey) Sign(rand io.Reader, msg []byte, opts crypto.SignerOpts) ([]byte, error) {
	signer := Signer{
		PrivateKey: *priv,
		Msg:        msg,
	}
	return signer.Sign()
}

func (priv *PrivateKey) Decrypt(data []byte) ([]byte, error) {
	return Decrypt(priv, data)
}

func (pub *PublicKey) Verify(msg []byte, sign []byte) bool {
	var sm2Sign sm2Signature

	_, err := asn1.Unmarshal(sign, &sm2Sign)
	if err != nil {
		return false
	}
	return Verify(pub, msg, sm2Sign.R, sm2Sign.S)
}

func (pub *PublicKey) Encrypt(data []byte) ([]byte, error) {
	return Encrypt(pub, data)
}

var one = new(big.Int).SetInt64(1)

func intToBytes(x int) []byte {
	var buf = make([]byte, 4)

	binary.BigEndian.PutUint32(buf, uint32(x))
	return buf
}

func kdf(x, y []byte, length int) ([]byte, bool) {
	var c []byte

	ct := 1
	h := sm3.New()
	x = append(x, y...)
	for i, j := 0, (length+31)/32; i < j; i++ {
		h.Reset()
		h.Write(x)
		h.Write(intToBytes(ct))
		hash := h.Sum(nil)
		if i+1 == j && length%32 != 0 {
			c = append(c, hash[:length%32]...)
		} else {
			c = append(c, hash...)
		}
		ct++
	}
	for i := 0; i < length; i++ {
		if c[i] != 0 {
			return c, true
		}
	}
	return c, false
}

func randFieldElement(c elliptic.Curve, rand io.Reader) (k *big.Int, err error) {
	params := c.Params()
	b := make([]byte, params.BitSize/8+8)
	_, err = io.ReadFull(rand, b)
	if err != nil {
		return
	}
	k = new(big.Int).SetBytes(b)
	n := new(big.Int).Sub(params.N, one)
	k.Mod(k, n)
	k.Add(k, one)
	return
}

func GenerateKey() (*PrivateKey, error) {
	c := P256Sm2()
	k, err := randFieldElement(c, rand.Reader)
	if err != nil {
		return nil, err
	}
	priv := new(PrivateKey)
	priv.PublicKey.Curve = c
	priv.D = k
	priv.PublicKey.X, priv.PublicKey.Y = c.ScalarBaseMult(k.Bytes())
	return priv, nil
}

var errZeroParam = errors.New("zero parameter")

func Verify(pub *PublicKey, hash []byte, r, s *big.Int) bool {
	c := pub.Curve
	N := c.Params().N

	if r.Sign() <= 0 || s.Sign() <= 0 {
		return false
	}
	if r.Cmp(N) >= 0 || s.Cmp(N) >= 0 {
		return false
	}

	// 调整算法细节以实现SM2
	t := new(big.Int).Add(r, s)
	t.Mod(t, N)
	if t.Sign() == 0 {
		return false
	}

	var x *big.Int
	x1, y1 := c.ScalarBaseMult(s.Bytes())
	x2, y2 := c.ScalarMult(pub.X, pub.Y, t.Bytes())
	x, _ = c.Add(x1, y1, x2, y2)

	e := new(big.Int).SetBytes(hash)
	x.Add(x, e)
	x.Mod(x, N)
	return x.Cmp(r) == 0
}

func Sm2Sign(priv *PrivateKey, msg, uid []byte) (r, s *big.Int, err error) {
	za, err := ZA(&priv.PublicKey, uid)
	if err != nil {
		return nil, nil, err
	}
	e, err := msgHash(za, msg)
	if err != nil {
		return nil, nil, err
	}
	c := priv.PublicKey.Curve
	N := c.Params().N
	if N.Sign() == 0 {
		return nil, nil, errZeroParam
	}
	var k *big.Int
	for { // 调整算法细节以实现SM2
		for {
			k, err = randFieldElement(c, rand.Reader)
			if err != nil {
				r = nil
				return
			}
			r, _ = priv.Curve.ScalarBaseMult(k.Bytes())
			r.Add(r, e)
			r.Mod(r, N)
			if r.Sign() != 0 {
				break
			}
			if t := new(big.Int).Add(r, k); t.Cmp(N) == 0 {
				break
			}
		}
		rD := new(big.Int).Mul(priv.D, r)
		s = new(big.Int).Sub(k, rD)
		d1 := new(big.Int).Add(priv.D, one)
		d1Inv := new(big.Int).ModInverse(d1, N)
		s.Mul(s, d1Inv)
		s.Mod(s, N)
		if s.Sign() != 0 {
			break
		}
	}
	return
}

func Sm2Verify(pub *PublicKey, msg, uid []byte, r, s *big.Int) bool {
	c := pub.Curve
	N := c.Params().N
	one := new(big.Int).SetInt64(1)
	if r.Cmp(one) < 0 || s.Cmp(one) < 0 {
		return false
	}
	if r.Cmp(N) >= 0 || s.Cmp(N) >= 0 {
		return false
	}
	za, err := ZA(pub, uid)
	if err != nil {
		return false
	}
	e, err := msgHash(za, msg)
	if err != nil {
		return false
	}
	t := new(big.Int).Add(r, s)
	t.Mod(t, N)
	if t.Sign() == 0 {
		return false
	}
	var x *big.Int
	x1, y1 := c.ScalarBaseMult(s.Bytes())
	x2, y2 := c.ScalarMult(pub.X, pub.Y, t.Bytes())
	x, _ = c.Add(x1, y1, x2, y2)

	x.Add(x, e)
	x.Mod(x, N)
	return x.Cmp(r) == 0
}

func msgHash(za, msg []byte) (*big.Int, error) {
	e := sm3.New()
	e.Write(za)
	e.Write(msg)
	return new(big.Int).SetBytes(e.Sum(nil)[:32]), nil
}

// ZA = H256(ENTLA || IDA || a || b || xG || yG || xA || yA)
func ZA(pub *PublicKey, uid []byte) ([]byte, error) {
	za := sm3.New()
	uidLen := len(uid)
	if uidLen >= 8192 {
		return []byte{}, errors.New("SM2: uid too large")
	}
	Entla := uint16(8 * uidLen)
	za.Write([]byte{byte((Entla >> 8) & 0xFF)})
	za.Write([]byte{byte(Entla & 0xFF)})
	za.Write(uid)
	za.Write(sm2P256ToBig(&sm2P256.a).Bytes())
	za.Write(sm2P256.B.Bytes())
	za.Write(sm2P256.Gx.Bytes())
	za.Write(sm2P256.Gy.Bytes())

	xBuf := pub.X.Bytes()
	yBuf := pub.Y.Bytes()
	if n := len(xBuf); n < 32 {
		xBuf = append(zeroByteSlice()[:32-n], xBuf...)
	}
	za.Write(xBuf)
	za.Write(yBuf)
	return za.Sum(nil)[:32], nil
}

// 32byte
func zeroByteSlice() []byte {
	return []byte{
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	}
}

/*
 * sm2密文结构如下:
 *  x
 *  y
 *  hash
 *  CipherText
 */
func Encrypt(pub *PublicKey, data []byte) ([]byte, error) {
	/*
		PB为公钥，M为明文，len为M的长度
		1. 产生随机数k，k的值大于等于1小于等于n-1
		2. 计算点C1 = k*G（点C1坐标对应x1, y1)
		3. 计算(x2, y2) = kPB
		4. 计算C2 = hash(x2||M||y2)，这里的hash采用SM3
		5. 计算ct = kdf(x2||y2, len)，若ct为全0则返回第一步
		6. 计算C3 = M⊕ct
		7. 密文C=C1||C2||C3
	*/
	if len(data) == 0 {
		return []byte{}, nil
	}
	length := len(data)
	for {
		c := []byte{}
		curve := pub.Curve
		k, err := randFieldElement(curve, rand.Reader)
		if err != nil {
			return nil, err
		}
		x1, y1 := curve.ScalarBaseMult(k.Bytes())
		x2, y2 := curve.ScalarMult(pub.X, pub.Y, k.Bytes())
		x1Buf := x1.Bytes()
		y1Buf := y1.Bytes()
		x2Buf := x2.Bytes()
		y2Buf := y2.Bytes()
		if n := len(x1Buf); n < 32 {
			x1Buf = append(zeroByteSlice()[:32-n], x1Buf...)
		}
		if n := len(y1Buf); n < 32 {
			y1Buf = append(zeroByteSlice()[:32-n], y1Buf...)
		}
		if n := len(x2Buf); n < 32 {
			x2Buf = append(zeroByteSlice()[:32-n], x2Buf...)
		}
		if n := len(y2Buf); n < 32 {
			y2Buf = append(zeroByteSlice()[:32-n], y2Buf...)
		}
		c = append(c, x1Buf...) // x分量
		c = append(c, y1Buf...) // y分量
		tm := []byte{}
		tm = append(tm, x2Buf...)
		tm = append(tm, data...)
		tm = append(tm, y2Buf...)
		h := sm3.Sm3Sum(tm)
		c = append(c, h...)
		ct, ok := kdf(x2Buf, y2Buf, length) // 密文
		if !ok {
			continue
		}
		c = append(c, ct...)
		for i := 0; i < length; i++ {
			c[96+i] ^= data[i]
		}
		return append([]byte{0x04}, c...), nil
	}
}

func Decrypt(priv *PrivateKey, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}
	data = data[1:]
	length := len(data) - 96
	curve := priv.Curve
	x := new(big.Int).SetBytes(data[:32])
	y := new(big.Int).SetBytes(data[32:64])
	x2, y2 := curve.ScalarMult(x, y, priv.D.Bytes())
	x2Buf := x2.Bytes()
	y2Buf := y2.Bytes()
	if n := len(x2Buf); n < 32 {
		x2Buf = append(zeroByteSlice()[:32-n], x2Buf...)
	}
	if n := len(y2Buf); n < 32 {
		y2Buf = append(zeroByteSlice()[:32-n], y2Buf...)
	}
	c, ok := kdf(x2Buf, y2Buf, length)
	if !ok {
		return nil, errors.New("Decrypt: failed to decrypt")
	}
	for i := 0; i < length; i++ {
		c[i] ^= data[i+96]
	}
	tm := []byte{}
	tm = append(tm, x2Buf...)
	tm = append(tm, c...)
	tm = append(tm, y2Buf...)
	h := sm3.Sm3Sum(tm)
	// TODO: 检查bytes.Compare函数和bytes.Equal哪个更加高效
	if bytes.Compare(h, data[64:96]) != 0 {
		return c, errors.New("Decrypt: failed to decrypt")
	}
	return c, nil
}

type zr struct {
	io.Reader
}

func (z *zr) Read(dst []byte) (n int, err error) {
	for i := range dst {
		dst[i] = 0
	}
	return len(dst), nil
}

var zeroReader = &zr{}

func getLastBit(a *big.Int) uint {
	return a.Bit(0)
}

func Compress(a *PublicKey) []byte {
	buf := []byte{}
	yp := getLastBit(a.Y)
	buf = append(buf, a.X.Bytes()...)
	if n := len(a.X.Bytes()); n < 32 {
		buf = append(zeroByteSlice()[:(32-n)], buf...)
	}
	buf = append([]byte{byte(yp)}, buf...)
	return buf
}

func Decompress(a []byte) *PublicKey {
	var aa, xx, xx3 sm2P256FieldElement

	P256Sm2()
	x := new(big.Int).SetBytes(a[1:])
	curve := sm2P256
	sm2P256FromBig(&xx, x)
	sm2P256Square(&xx3, &xx)       // x3 = x ^ 2
	sm2P256Mul(&xx3, &xx3, &xx)    // x3 = x ^ 2 * x
	sm2P256Mul(&aa, &curve.a, &xx) // a = a * x
	sm2P256Add(&xx3, &xx3, &aa)
	sm2P256Add(&xx3, &xx3, &curve.b)

	y2 := sm2P256ToBig(&xx3)
	y := new(big.Int).ModSqrt(y2, sm2P256.P)
	if getLastBit(y) != uint(a[0]) {
		y.Sub(sm2P256.P, y)
	}
	return &PublicKey{
		Curve: P256Sm2(),
		X:     x,
		Y:     y,
	}
}

func SignDigitToSignData(r, s *big.Int) ([]byte, error) {
	return asn1.Marshal(sm2Signature{r, s})
}

func SignDataToSignDigit(sign []byte) (*big.Int, *big.Int, error) {
	var sm2Sign sm2Signature

	_, err := asn1.Unmarshal(sign, &sm2Sign)
	if err != nil {
		return nil, nil, err
	}

	return sm2Sign.R, sm2Sign.S, nil
}
