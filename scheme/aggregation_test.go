// Copyright 2021 Nym Technologies SA
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package coconut

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/consensys/gurvy/bls381"
	"github.com/goconut/utils"
	"github.com/stretchr/testify/assert"
	"gitlab.nymte.ch/nym/coconut/coconutGo"
)

// just helpers
func randomSignature() *Signature {
	params, err := coconutGo.Setup(1)
	unwrapError(err)

	r, err := params.RandomScalar()
	unwrapError(err)

	s, err := params.RandomScalar()
	unwrapError(err)

	g1, _, _, _ := bls381.Generators()

	return &Signature{
		sig1: utils.G1ScalarMul(&g1, &r),
		sig2: utils.G1ScalarMul(&g1, &s),
	}
}

func randomVerificationKey(size int) *VerificationKey {
	params, err := coconutGo.Setup(1)
	unwrapError(err)

	r, err := params.RandomScalar()
	unwrapError(err)

	_, g2, _, _ := bls381.Generators()

	alpha := utils.G2ScalarMul(&g2, &r)
	beta := make([]*bls381.G2Jac, size)
	for i := 0; i < size; i++ {
		r, err := params.RandomScalar()
		unwrapError(err)
		betai := utils.G2ScalarMul(&g2, &r)
		beta[i] = &betai
	}

	return &VerificationKey{
		alpha: alpha,
		beta:  beta,
	}
}

func TestKeyAggregationOfAnyKeySubset(t *testing.T) {
	params, err := coconutGo.Setup(4)
	unwrapError(err)

	keypairs, err := TTPKeygen(params, 3, 5)
	unwrapError(err)

	verificationKeys := make([]*VerificationKey, 5)
	for i := 0; i < 5; i++ {
		verificationKeys[i] = &keypairs[i].VerificationKey
	}

	aggrVk1, err := AggregateVerificationKeys(verificationKeys[:3], []SignerIndex{1, 2, 3})
	unwrapError(err)

	aggrVk2, err := AggregateVerificationKeys(verificationKeys[2:], []SignerIndex{3, 4, 5})
	unwrapError(err)
	assert.True(t, aggrVk1.Equal(&aggrVk2))

	// aggregating threshold+1
	aggrMore, err := AggregateVerificationKeys(verificationKeys[1:], []SignerIndex{2, 3, 4, 5})
	unwrapError(err)
	assert.True(t, aggrVk1.Equal(&aggrMore))

	// aggregating all
	aggrAll, err := AggregateVerificationKeys(verificationKeys, []SignerIndex{1, 2, 3, 4, 5})
	unwrapError(err)
	assert.True(t, aggrVk1.Equal(&aggrAll))

	// not taking enough points (threshold was 3)
	aggrNotEnough, err := AggregateVerificationKeys(verificationKeys[:2], []SignerIndex{1, 2})
	unwrapError(err)
	assert.False(t, aggrVk1.Equal(&aggrNotEnough))

	// taking wrong Index
	aggrBad, err := AggregateVerificationKeys(verificationKeys[2:], []SignerIndex{42, 123, 100})
	unwrapError(err)
	assert.False(t, aggrVk1.Equal(&aggrBad))
}

func TestEmptyKeySubsetAggregation(t *testing.T) {
	keys := make([]*VerificationKey, 0)
	_, err := AggregateVerificationKeys(keys, []SignerIndex{})
	assert.Error(t, err)
}

func TestKeyAggregationWithInvalidIndices(t *testing.T) {
	keys := []*VerificationKey{randomVerificationKey(3)}
	_, err := AggregateVerificationKeys(keys, []SignerIndex{})
	assert.Error(t, err)

	_, err = AggregateVerificationKeys(keys, []SignerIndex{1, 2})
	assert.Error(t, err)
}

func TestKeyAggregationWithNonuniqueIndices(t *testing.T) {
	keys := []*VerificationKey{randomVerificationKey(3), randomVerificationKey(3)}
	_, err := AggregateVerificationKeys(keys, []SignerIndex{1, 1})
	assert.Error(t, err)
}

func TestKeyAggregationOfDifferentKeySizes(t *testing.T) {
	keys := []*VerificationKey{randomVerificationKey(3), randomVerificationKey(1)}
	_, err := AggregateVerificationKeys(keys, []SignerIndex{})
	assert.Error(t, err)
}

func TestSignatureAggregationForAnySignatureSubset(t *testing.T) {
	params, err := coconutGo.Setup(2)
	unwrapError(err)

	attributes, err := params.NRandomScalars(2)
	unwrapError(err)

	keypairs, err := TTPKeygen(params, 3, 5)
	unwrapError(err)

	sigs := make([]*Signature, 5)
	for i := 0; i < 5; i++ {
		sig, err := Sign(params, &keypairs[i].SecretKey, attributes)
		unwrapError(err)
		sigs[i] = &sig
	}

	verificationKeys := make([]*VerificationKey, 5)
	for i := 0; i < 5; i++ {
		verificationKeys[i] = &keypairs[i].VerificationKey
	}

	aggrSig1, err := AggregateSignatures(sigs[:3], []SignerIndex{1, 2, 3})
	unwrapError(err)

	aggrSig2, err := AggregateSignatures(sigs[2:], []SignerIndex{3, 4, 5})
	unwrapError(err)
	assert.True(t, aggrSig1.Equal(&aggrSig2))

	// verify credential for good measure
	aggrVk, err := AggregateVerificationKeys(verificationKeys[:3], []SignerIndex{1, 2, 3})
	unwrapError(err)
	assert.True(t, Verify(params, &aggrVk, attributes, &aggrSig1))

	// aggregating threshold+1
	aggrMore, err := AggregateSignatures(sigs[1:], []SignerIndex{2, 3, 4, 5})
	unwrapError(err)
	assert.True(t, aggrSig1.Equal(&aggrMore))

	// aggregating all
	aggrAll, err := AggregateSignatures(sigs, []SignerIndex{1, 2, 3, 4, 5})
	unwrapError(err)
	assert.True(t, aggrSig1.Equal(&aggrAll))

	// not taking enough points (threshold was 3)
	aggrNotEnough, err := AggregateSignatures(sigs[:2], []SignerIndex{1, 2})
	unwrapError(err)
	assert.False(t, aggrSig1.Equal(&aggrNotEnough))

	// taking wrong Index
	aggrBad, err := AggregateSignatures(sigs[2:], []SignerIndex{42, 123, 100})
	unwrapError(err)
	assert.False(t, aggrSig1.Equal(&aggrBad))

	// checking random permutations
	for i := 0; i < 100; i++ {
		indices := []SignerIndex{1, 2, 3, 4, 5}
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(indices), func(i, j int) { indices[i], indices[j] = indices[j], indices[i] })
		sigsLocal := make([]*Signature, 3)
		sigsLocal[0] = sigs[int(indices[0])-1]
		sigsLocal[1] = sigs[int(indices[1])-1]
		sigsLocal[2] = sigs[int(indices[2])-1]

		aggr, err := AggregateSignatures(sigsLocal, indices[:3])
		unwrapError(err)
		assert.True(t, aggrSig1.Equal(&aggr), fmt.Sprintf("%v X:%v\nY:%v\nZ:%v", indices[:3], aggr.sig2.X, aggr.sig2.Y, aggr.sig2.Z))
	}

	sigs2 := []*Signature{
		sigs[3], sigs[1], sigs[0],
	}
	idx := []SignerIndex{4, 2, 1}
	aggr, err := AggregateSignatures(sigs2, idx)
	unwrapError(err)

	var aggrAff bls381.G1Affine
	aggrAff.FromJacobian(&aggr.sig2)

	assert.True(t, aggrSig1.Equal(&aggr), fmt.Sprintf("%v X:%v\nY:%v\n", idx, aggrAff.X, aggrAff.Y))
}

func TestEmptySignatureSubsetAggregation(t *testing.T) {
	keys := make([]*Signature, 0)
	_, err := AggregateSignatures(keys, []SignerIndex{})
	assert.Error(t, err)
}

func TestSignatureAggregationWithInvalidIndices(t *testing.T) {
	keys := []*Signature{randomSignature()}
	_, err := AggregateSignatures(keys, []SignerIndex{})
	assert.Error(t, err)

	_, err = AggregateSignatures(keys, []SignerIndex{1, 2})
	assert.Error(t, err)
}

func TestSignatureAggregationWithNonuniqueIndices(t *testing.T) {
	keys := []*Signature{randomSignature(), randomSignature()}
	_, err := AggregateSignatures(keys, []SignerIndex{1, 1})
	assert.Error(t, err)
}

// TODO: test for aggregating non-threshold keys
