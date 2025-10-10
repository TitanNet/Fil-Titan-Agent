package wallet

import (
	"fmt"
	"io"
	"os"

	ckeys "github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/go-bip39"
	ethhd "github.com/tharsis/ethermint/crypto/hd"
)

const (
	DefaultKeyName       = "titan"
	DefaultChainID       = "titan"
	DefaultAccountPrefix = "titan"
	DefaultBackend       = keyring.BackendTest
	DefaultCoinType      = sdk.CoinType
)

// Option defines a function keys options for the ethereum Secp256k1 curve.
// It supports secp256k1 and eth_secp256k1 keys for accounts.
func lensKeyringAlgoOptions() keyring.Option {
	return func(options *keyring.Options) {
		options.SupportedAlgos = keyring.SigningAlgoList{hd.Secp256k1, ethhd.EthSecp256k1}
		options.SupportedAlgosLedger = keyring.SigningAlgoList{hd.Secp256k1, ethhd.EthSecp256k1}
	}
}

type WalletOptions struct {
	ChainID        string
	AccountPrefix  string
	KeyringBackend string
	KeyDirectory   string
	Input          io.Reader
}

type WalletOption func(*WalletOptions)

func WithChainID(chainID string) WalletOption {
	return func(opts *WalletOptions) {
		opts.ChainID = chainID
	}
}

func WithAccountPrefix(prefix string) WalletOption {
	return func(opts *WalletOptions) {
		opts.AccountPrefix = prefix
	}
}

func WithKeyringBackend(backend string) WalletOption {
	return func(opts *WalletOptions) {
		opts.KeyringBackend = backend
	}
}

func WithKeyDirectory(dir string) WalletOption {
	return func(opts *WalletOptions) {
		opts.KeyDirectory = dir
	}
}

func WithInput(input io.Reader) WalletOption {
	return func(opts *WalletOptions) {
		opts.Input = input
	}
}

type KeyOutput struct {
	Mnemonic string `json:"mnemonic" yaml:"mnemonic"`
	Address  string `json:"address" yaml:"address"`
}

// Wallet 封装钱包相关操作
type Wallet struct {
	keyring       keyring.Keyring
	accountPrefix string
}

// NewWallet 初始化钱包管理器
func NewWallet(options ...WalletOption) (*Wallet, error) {
	// 设置默认值
	opts := &WalletOptions{
		ChainID:        "titan",
		AccountPrefix:  "titan",
		KeyringBackend: "os",
		KeyDirectory:   "./keys",
		Input:          os.Stdin,
	}

	// 应用选项函数
	for _, opt := range options {
		opt(opts)
	}

	keybase, err := keyring.New(opts.ChainID, opts.KeyringBackend, opts.KeyDirectory, opts.Input, lensKeyringAlgoOptions())
	if err != nil {
		return nil, err
	}

	return &Wallet{keyring: keybase, accountPrefix: opts.AccountPrefix}, nil
}

func (tw *Wallet) AddKey(keyName string, coinType uint32, mnemonic ...string) (output *KeyOutput, err error) {
	var mnemonicStr string
	// var err error
	var info keyring.Info
	algo := keyring.SignatureAlgo(hd.Secp256k1)

	if len(mnemonic) > 0 {
		mnemonicStr = mnemonic[0]
	} else {
		// mnemonicStr, err = createMnemonic()
		entropySeed, err := bip39.NewEntropy(256)
		if err != nil {
			return nil, err
		}
		mnemonicStr, err = bip39.NewMnemonic(entropySeed)
		if err != nil {
			return nil, err
		}
	}

	if coinType == 60 {
		algo = keyring.SignatureAlgo(ethhd.EthSecp256k1)
	}

	info, err = tw.keyring.NewAccount(keyName, mnemonicStr, "", hd.CreateHDPath(coinType, 0, 0).String(), algo)
	if err != nil {
		return nil, err
	}

	out, err := sdk.Bech32ifyAddressBytes(tw.accountPrefix, info.GetAddress())
	if err != nil {
		return nil, err
	}
	return &KeyOutput{Mnemonic: mnemonicStr, Address: out}, nil
}

func (tw *Wallet) DeleteKey(keyName string) error {
	return tw.keyring.Delete(keyName)
}

// ImportFromMnemonic 从助记词导入钱包
func (tw *Wallet) ImportFromMnemonic(name, mnemonic string) error {
	_, err := tw.AddKey(name, sdk.CoinType, mnemonic)
	if err != nil {
		return err
	}
	return nil
}

func (tw *Wallet) ExportPrivKeyArmor(keyName string) (armor string, err error) {
	return tw.keyring.ExportPrivKeyArmor(keyName, ckeys.DefaultKeyPass)
}

// GetAddress 获取钱包地址
func (tw *Wallet) GetAddress(name string) (string, error) {
	info, err := tw.keyring.Key(name)
	if err != nil {
		return "", err
	}

	return sdk.Bech32ifyAddressBytes(tw.accountPrefix, info.GetAddress())
}

func (tw *Wallet) GetPubKey(name string) (types.PubKey, error) {
	info, err := tw.keyring.Key(name)
	if err != nil {
		return nil, err
	}

	return info.GetPubKey(), nil
}

func (tw *Wallet) ListKeys() ([]keyring.Info, error) {
	return tw.keyring.List()
}

func (tw *Wallet) Sign(keyName string, bytes []byte) ([]byte, error) {
	_, err := tw.keyring.Key(keyName)
	if err != nil {
		return nil, err
	}

	sig, _, err := tw.keyring.Sign(keyName, bytes)
	return sig, err
}

func (tw *Wallet) VerifySignature(keyName string, msg []byte, signature []byte) error {
	info, err := tw.keyring.Key(keyName)
	if err != nil {
		return err
	}

	if !info.GetPubKey().VerifySignature(msg, signature) {
		return fmt.Errorf("verify signature failed")
	}
	return nil
}
