package wallet

import (
	"encoding/hex"
	"fmt"
	"log"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

func TestWallet(t *testing.T) {
	wallet, err := NewWallet(
		WithChainID("titan"),
		WithAccountPrefix("titan"),
		WithKeyringBackend(keyring.BackendOS),
		WithKeyDirectory("D://codes//titan-agent-private//common"),
	)
	if err != nil {
		log.Fatalf("Create wallet failed: %v", err)
	}

	keys, err := wallet.ListKeys()
	if err != nil {
		log.Fatalf("list keys: %v", err)
	}

	for _, key := range keys {
		// fmt.Printf("%s:%s\n", key.GetName(), key.GetAddress().String())
		wallet.DeleteKey(key.GetName())
	}

	key, err := wallet.AddKey("abc", sdk.CoinType)
	if err != nil {
		log.Fatalf("AddKey: %v", err)
	}

	fmt.Printf("Wallet Address:%s\n", key.Address)
	fmt.Printf("Mnemonic:%s\n", key.Mnemonic)

	testData := []byte("hello, cosmos")
	sig, err := wallet.Sign("abc", testData)
	if err != nil {
		log.Fatalf("Sign failed: %v", err)
	}

	fmt.Printf("sign: %s\n", hex.EncodeToString(sig))

	pubKey, err := wallet.GetPubKey("abc")
	if err != nil {
		log.Fatalf("GetPubKey: %v", err)
	}

	fmt.Println("Public key hex:", hex.EncodeToString(pubKey.Bytes()))
	pubKeyBytes := pubKey.Bytes()
	var newPubKey = secp256k1.PubKey(pubKeyBytes)

	if newPubKey.VerifySignature(testData, sig) {
		fmt.Println("verify sign success")
	} else {
		fmt.Println("verify sign failed")
	}

	address := sdk.AccAddress(newPubKey.Address())
	_, err = sdk.Bech32ifyAddressBytes("titan", address)
	if err != nil {
		log.Fatalf("Bech32ifyAddressBytes: %v", err)
	}
}
