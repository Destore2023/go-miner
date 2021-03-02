package rpc

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Sukhavati-Labs/go-miner/poc/wallet"
	"github.com/Sukhavati-Labs/go-miner/poc/wallet/keystore"
	"io"
	"os"

	"github.com/Sukhavati-Labs/go-miner/logging"
	pb "github.com/Sukhavati-Labs/go-miner/rpc/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/status"
)

var (
	keystoreFileNamePrefix = "keystore"
)

func (s *Server) GetKeystore(ctx context.Context, msg *empty.Empty) (*pb.GetKeystoreResponse, error) {
	keystores := make([]*pb.WalletSummary, 0)
	for _, summary := range s.pocWallet.GetManagedAddrManager() {
		keystores = append(keystores, &pb.WalletSummary{
			WalletId: summary.Name(),
			Remark:   summary.Remarks(),
		})
	}
	return &pb.GetKeystoreResponse{
		Wallets: keystores,
	}, nil
}

func getKeystoreDetail(keystore *keystore.Keystore) *pb.PocWallet {

	walletCrypto := &pb.WalletCrypto{
		Cipher:             keystore.Crypto.Cipher,
		MasterHDPrivKeyEnc: keystore.Crypto.MasterHDPrivKeyEnc,
		KDF:                keystore.Crypto.KDF,
		PubParams:          keystore.Crypto.PubParams,
		PrivParams:         keystore.Crypto.PrivParams,
		CryptoKeyPubEnc:    keystore.Crypto.CryptoKeyPubEnc,
		CryptoKeyPrivEnc:   keystore.Crypto.CryptoKeyPrivEnc,
	}
	HdWalletPath := &pb.HDWalletPath{
		Purpose:          keystore.HDpath.Purpose,
		Coin:             keystore.HDpath.Coin,
		Account:          keystore.HDpath.Account,
		ExternalChildNum: keystore.HDpath.ExternalChildNum,
		InternalChildNum: keystore.HDpath.InternalChildNum,
	}

	return &pb.PocWallet{
		Remark: keystore.Remark,
		Crypto: walletCrypto,
		HDPath: HdWalletPath,
	}
}

func (s *Server) GetKeystoreDetail(ctx context.Context, in *pb.GetKeystoreDetailRequest) (*pb.GetKeystoreDetailResponse, error) {
	keystore, addrManager, err := s.pocWallet.ExportKeystore(in.WalletId, []byte(in.Passphrase))
	if err != nil {
		return nil, err
	}
	addrManager.KeyScope()
	detail := getKeystoreDetail(keystore)
	detail.WalletId = in.WalletId
	addresses := make([]*pb.PocAddress, 0)
	for _, addr := range addrManager.ManagedAddresses() {
		HdPath := addr.DerivationPath()
		address := &pb.PocAddress{
			PubKey:     hex.EncodeToString(addr.PubKey().SerializeCompressed()),
			ScriptHash: hex.EncodeToString(addr.ScriptAddress()),
			Address:    addr.String(),
			DerivationPath: &pb.DerivationPath{
				Account: HdPath.Account,
				Branch:  HdPath.Branch,
				Index:   HdPath.Index,
			},
		}
		if addr.PrivKey() != nil {
			address.PrivKey = hex.EncodeToString(addr.PrivKey().Serialize())
		}
		addresses = append(addresses, address)
	}
	detail.AddrManager = &pb.AddrManager{
		KeystoreName: addrManager.Name(),
		Remark:       addrManager.Remarks(),
		Expires:      0,
		Addresses:    addresses,
		Use:          uint32(addrManager.AddrUse()),
	}

	return &pb.GetKeystoreDetailResponse{
		WalletId: in.WalletId,
		Wallet:   detail,
	}, nil
}

func (s *Server) ExportKeystore(ctx context.Context, in *pb.ExportKeystoreRequest) (*pb.ExportKeystoreResponse, error) {
	logging.CPrint(logging.INFO, "a request is received to export keystore", logging.LogFormat{"export keystore id": in.WalletId})
	err := checkWalletIdLen(in.WalletId)
	if err != nil {
		return nil, err
	}
	err = checkPassLen(in.Passphrase)
	if err != nil {
		return nil, err
	}
	// get keystore json from wallet
	keystore, _, err := s.pocWallet.ExportKeystore(in.WalletId, []byte(in.Passphrase))
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIExportWallet], logging.LogFormat{
			"err": err,
		})
		return nil, status.New(ErrAPIExportWallet, ErrCode[ErrAPIExportWallet]).Err()
	}
	keystoreJSON := keystore.Bytes()
	// write keystore json file to disk
	exportFileName := fmt.Sprintf("%s/%s-%s.json", in.ExportPath, keystoreFileNamePrefix, in.WalletId)
	file, err := os.OpenFile(exportFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIOpenFile], logging.LogFormat{
			"error": err,
		})
		return nil, status.New(ErrAPIOpenFile, ErrCode[ErrAPIOpenFile]).Err()
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	_, err = writer.WriteString(string(keystoreJSON))
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIWriteFile], logging.LogFormat{
			"error": err,
		})
		return nil, status.New(ErrAPIWriteFile, ErrCode[ErrAPIWriteFile]).Err()
	}
	err = writer.Flush()
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIFlush], logging.LogFormat{
			"error": err,
		})
		return nil, status.New(ErrAPIFlush, ErrCode[ErrAPIFlush]).Err()
	}

	logging.CPrint(logging.INFO, "the request to export keystore was successfully answered", logging.LogFormat{"export keystore id": in.WalletId})
	return &pb.ExportKeystoreResponse{
		Keystore: string(keystoreJSON),
	}, nil
}

func (s *Server) ExportKeystoreByDir(ctx context.Context, in *pb.ExportKeystoreByDirRequest) (*pb.ExportKeystoreByDirResponse, error) {
	logging.CPrint(logging.INFO, "rpc ExportKeystoreByDirs called")
	pocWalletConfig := wallet.NewPocWalletConfig(in.WalletDir, "leveldb")
	exportWallet, err := wallet.NewPoCWallet(pocWalletConfig, []byte(in.Passphrase))
	if err != nil {
		return nil, err
	}
	defer exportWallet.Close()
	keystores, err := exportWallet.ExportKeystores([]byte(in.WalletPassphrase))
	if err != nil {
		return nil, err
	}
	keystoreJSON, err := json.Marshal(keystores)
	if err != nil {
		return nil, err
	}
	// write keystore json file to disk
	exportFileName := fmt.Sprintf("%s/%s-all.json", in.ExportPath, keystoreFileNamePrefix)
	file, err := os.OpenFile(exportFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIOpenFile], logging.LogFormat{
			"error": err,
		})
		return nil, status.New(ErrAPIOpenFile, ErrCode[ErrAPIOpenFile]).Err()
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	_, err = writer.WriteString(string(keystoreJSON))
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIWriteFile], logging.LogFormat{
			"error": err,
		})
		return nil, status.New(ErrAPIWriteFile, ErrCode[ErrAPIWriteFile]).Err()
	}
	err = writer.Flush()
	if err != nil {
		logging.CPrint(logging.ERROR, ErrCode[ErrAPIFlush], logging.LogFormat{
			"error": err,
		})
		return nil, status.New(ErrAPIFlush, ErrCode[ErrAPIFlush]).Err()
	}
	logging.CPrint(logging.INFO, "the request to export keystore was successfully answered")
	return &pb.ExportKeystoreByDirResponse{
		Keystore: string(keystoreJSON),
	}, nil
}

func (s *Server) ImportKeystore(ctx context.Context, in *pb.ImportKeystoreRequest) (*pb.ImportKeystoreResponse, error) {
	err := checkPassLen(in.OldPassphrase)
	if err != nil {
		return nil, err
	}
	if len(in.NewPassphrase) != 0 {
		err = checkPassLen(in.NewPassphrase)
		if err != nil {
			return nil, err
		}
	}

	file, err := os.Open(in.ImportPath)
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to open file", logging.LogFormat{"file": in.ImportPath, "error": err})
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	fileBytes, err := reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		logging.CPrint(logging.ERROR, "failed to read file", logging.LogFormat{"error": err})
		return nil, err
	}

	accountID, remark, err := s.pocWallet.ImportKeystore(fileBytes, []byte(in.OldPassphrase), []byte(in.NewPassphrase))
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to import keystore", logging.LogFormat{"error": err})
		return nil, err
	}
	return &pb.ImportKeystoreResponse{
		Status:   true,
		WalletId: accountID,
		Remark:   remark,
	}, nil
}

func (s *Server) ImportKeystoreByDir(ctx context.Context, in *pb.ImportKeystoreByDirRequest) (*pb.ImportKeystoreByDirResponse, error) {
	logging.CPrint(logging.INFO, "rpc ImportKeystoreByDirs called")
	pocWalletConfig := wallet.NewPocWalletConfig(in.ImportKeystoreDir, "leveldb")
	exportWallet, err := wallet.NewPoCWallet(pocWalletConfig, []byte(in.ImportPubpass))
	if err != nil {
		return nil, err
	}
	defer exportWallet.Close()
	keystores, err := exportWallet.ExportKeystores([]byte(in.ImportPrivpass))
	if err != nil {
		return nil, err
	}
	for _, keystoreJson := range keystores {
		_, _, err := s.pocWallet.ImportKeystore([]byte(keystoreJson), []byte(in.ImportPrivpass), []byte(in.NewPrivpass))
		if err != nil {
			return nil, err
		}
	}

	keystoreJSON, err := json.Marshal(keystores)
	if err != nil {
		return nil, err
	}
	return &pb.ImportKeystoreByDirResponse{
		Keystore: string(keystoreJSON),
	}, nil
}

func (s *Server) UnlockWallet(ctx context.Context, in *pb.UnlockWalletRequest) (*pb.UnlockWalletResponse, error) {
	logging.CPrint(logging.INFO, "rpc unlock wallet called")
	err := checkPassLen(in.Passphrase)
	if err != nil {
		return nil, err
	}
	resp := &pb.UnlockWalletResponse{}

	if !s.pocWallet.IsLocked() {
		logging.CPrint(logging.INFO, "rpc unlock wallet succeed")
		resp.Success, resp.Error = true, ""
		return resp, nil
	}

	if err := s.pocWallet.Unlock([]byte(in.Passphrase)); err != nil {
		logging.CPrint(logging.ERROR, "rpc unlock wallet failed", logging.LogFormat{"err": err})
		return nil, status.New(ErrAPIWalletInternal, err.Error()).Err()
	}

	logging.CPrint(logging.INFO, "rpc unlock wallet succeed")
	resp.Success, resp.Error = true, ""
	return resp, nil
}

func (s *Server) LockWallet(ctx context.Context, msg *empty.Empty) (*pb.LockWalletResponse, error) {
	logging.CPrint(logging.INFO, "rpc lock wallet called")
	resp := &pb.LockWalletResponse{}

	if s.pocWallet.IsLocked() {
		logging.CPrint(logging.INFO, "rpc lock wallet succeed")
		resp.Success, resp.Error = true, ""
		return resp, nil
	}

	if s.pocMiner.Started() {
		logging.CPrint(logging.ERROR, "rpc lock wallet failed", logging.LogFormat{"err": ErrCode[ErrAPIWalletIsMining]})
		return nil, status.New(ErrAPIWalletIsMining, ErrCode[ErrAPIWalletIsMining]).Err()
	}

	s.pocWallet.Lock()
	logging.CPrint(logging.INFO, "rpc lock wallet succeed")
	resp.Success, resp.Error = true, ""
	return resp, nil
}

func (s *Server) ChangePrivatePass(ctx context.Context, in *pb.ChangePrivatePassRequest) (*pb.ChangePrivatePassResponse, error) {
	err := checkPassLen(in.OldPrivpass)
	if err != nil {
		return nil, err
	}
	err = checkPassLen(in.NewPrivpass)
	if err != nil {
		return nil, err
	}
	err = s.pocWallet.ChangePrivPassphrase([]byte(in.OldPrivpass), []byte(in.NewPrivpass), nil)
	if err != nil {
		return nil, err
	}
	return &pb.ChangePrivatePassResponse{
		Success: true,
	}, nil
}

func (s *Server) ChangePublicPass(ctx context.Context, in *pb.ChangePublicPassRequest) (*pb.ChangePublicPassResponse, error) {
	err := checkPassLen(in.OldPubpass)
	if err != nil {
		return nil, err
	}
	err = checkPassLen(in.NewPubpass)
	if err != nil {
		return nil, err
	}
	err = s.pocWallet.ChangePubPassphrase([]byte(in.OldPubpass), []byte(in.NewPubpass), nil)
	if err != nil {
		return nil, err
	}
	return &pb.ChangePublicPassResponse{
		Success: true,
	}, nil
}
