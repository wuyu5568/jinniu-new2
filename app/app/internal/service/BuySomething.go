// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package service

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// BuySomethingMetaData contains all meta data concerning the BuySomething contract.
var BuySomethingMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"num\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"}],\"name\":\"buy\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"startIndex\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"endIndex\",\"type\":\"uint256\"}],\"name\":\"getIdsByIndex\",\"outputs\":[{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getUserLength\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getUsers\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"startIndex\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"endIndex\",\"type\":\"uint256\"}],\"name\":\"getUsersAmountByIndex\",\"outputs\":[{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"startIndex\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"endIndex\",\"type\":\"uint256\"}],\"name\":\"getUsersByIndex\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"ids\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usdt\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"users\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"usersAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// BuySomethingABI is the input ABI used to generate the binding from.
// Deprecated: Use BuySomethingMetaData.ABI instead.
var BuySomethingABI = BuySomethingMetaData.ABI

// BuySomething is an auto generated Go binding around an Ethereum contract.
type BuySomething struct {
	BuySomethingCaller     // Read-only binding to the contract
	BuySomethingTransactor // Write-only binding to the contract
	BuySomethingFilterer   // Log filterer for contract events
}

// BuySomethingCaller is an auto generated read-only Go binding around an Ethereum contract.
type BuySomethingCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BuySomethingTransactor is an auto generated write-only Go binding around an Ethereum contract.
type BuySomethingTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BuySomethingFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type BuySomethingFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BuySomethingSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type BuySomethingSession struct {
	Contract     *BuySomething     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// BuySomethingCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type BuySomethingCallerSession struct {
	Contract *BuySomethingCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// BuySomethingTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type BuySomethingTransactorSession struct {
	Contract     *BuySomethingTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// BuySomethingRaw is an auto generated low-level Go binding around an Ethereum contract.
type BuySomethingRaw struct {
	Contract *BuySomething // Generic contract binding to access the raw methods on
}

// BuySomethingCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type BuySomethingCallerRaw struct {
	Contract *BuySomethingCaller // Generic read-only contract binding to access the raw methods on
}

// BuySomethingTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type BuySomethingTransactorRaw struct {
	Contract *BuySomethingTransactor // Generic write-only contract binding to access the raw methods on
}

// NewBuySomething creates a new instance of BuySomething, bound to a specific deployed contract.
func NewBuySomething(address common.Address, backend bind.ContractBackend) (*BuySomething, error) {
	contract, err := bindBuySomething(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &BuySomething{BuySomethingCaller: BuySomethingCaller{contract: contract}, BuySomethingTransactor: BuySomethingTransactor{contract: contract}, BuySomethingFilterer: BuySomethingFilterer{contract: contract}}, nil
}

// NewBuySomethingCaller creates a new read-only instance of BuySomething, bound to a specific deployed contract.
func NewBuySomethingCaller(address common.Address, caller bind.ContractCaller) (*BuySomethingCaller, error) {
	contract, err := bindBuySomething(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &BuySomethingCaller{contract: contract}, nil
}

// NewBuySomethingTransactor creates a new write-only instance of BuySomething, bound to a specific deployed contract.
func NewBuySomethingTransactor(address common.Address, transactor bind.ContractTransactor) (*BuySomethingTransactor, error) {
	contract, err := bindBuySomething(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &BuySomethingTransactor{contract: contract}, nil
}

// NewBuySomethingFilterer creates a new log filterer instance of BuySomething, bound to a specific deployed contract.
func NewBuySomethingFilterer(address common.Address, filterer bind.ContractFilterer) (*BuySomethingFilterer, error) {
	contract, err := bindBuySomething(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &BuySomethingFilterer{contract: contract}, nil
}

// bindBuySomething binds a generic wrapper to an already deployed contract.
func bindBuySomething(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(BuySomethingABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BuySomething *BuySomethingRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BuySomething.Contract.BuySomethingCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BuySomething *BuySomethingRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BuySomething.Contract.BuySomethingTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BuySomething *BuySomethingRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BuySomething.Contract.BuySomethingTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BuySomething *BuySomethingCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BuySomething.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BuySomething *BuySomethingTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BuySomething.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BuySomething *BuySomethingTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BuySomething.Contract.contract.Transact(opts, method, params...)
}

// GetIdsByIndex is a free data retrieval call binding the contract method 0x4f94b243.
//
// Solidity: function getIdsByIndex(uint256 startIndex, uint256 endIndex) view returns(uint256[])
func (_BuySomething *BuySomethingCaller) GetIdsByIndex(opts *bind.CallOpts, startIndex *big.Int, endIndex *big.Int) ([]*big.Int, error) {
	var out []interface{}
	err := _BuySomething.contract.Call(opts, &out, "getIdsByIndex", startIndex, endIndex)

	if err != nil {
		return *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]*big.Int)).(*[]*big.Int)

	return out0, err

}

// GetIdsByIndex is a free data retrieval call binding the contract method 0x4f94b243.
//
// Solidity: function getIdsByIndex(uint256 startIndex, uint256 endIndex) view returns(uint256[])
func (_BuySomething *BuySomethingSession) GetIdsByIndex(startIndex *big.Int, endIndex *big.Int) ([]*big.Int, error) {
	return _BuySomething.Contract.GetIdsByIndex(&_BuySomething.CallOpts, startIndex, endIndex)
}

// GetIdsByIndex is a free data retrieval call binding the contract method 0x4f94b243.
//
// Solidity: function getIdsByIndex(uint256 startIndex, uint256 endIndex) view returns(uint256[])
func (_BuySomething *BuySomethingCallerSession) GetIdsByIndex(startIndex *big.Int, endIndex *big.Int) ([]*big.Int, error) {
	return _BuySomething.Contract.GetIdsByIndex(&_BuySomething.CallOpts, startIndex, endIndex)
}

// GetUserLength is a free data retrieval call binding the contract method 0x7456fed6.
//
// Solidity: function getUserLength() view returns(uint256)
func (_BuySomething *BuySomethingCaller) GetUserLength(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _BuySomething.contract.Call(opts, &out, "getUserLength")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetUserLength is a free data retrieval call binding the contract method 0x7456fed6.
//
// Solidity: function getUserLength() view returns(uint256)
func (_BuySomething *BuySomethingSession) GetUserLength() (*big.Int, error) {
	return _BuySomething.Contract.GetUserLength(&_BuySomething.CallOpts)
}

// GetUserLength is a free data retrieval call binding the contract method 0x7456fed6.
//
// Solidity: function getUserLength() view returns(uint256)
func (_BuySomething *BuySomethingCallerSession) GetUserLength() (*big.Int, error) {
	return _BuySomething.Contract.GetUserLength(&_BuySomething.CallOpts)
}

// GetUsers is a free data retrieval call binding the contract method 0x00ce8e3e.
//
// Solidity: function getUsers() view returns(address[])
func (_BuySomething *BuySomethingCaller) GetUsers(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _BuySomething.contract.Call(opts, &out, "getUsers")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetUsers is a free data retrieval call binding the contract method 0x00ce8e3e.
//
// Solidity: function getUsers() view returns(address[])
func (_BuySomething *BuySomethingSession) GetUsers() ([]common.Address, error) {
	return _BuySomething.Contract.GetUsers(&_BuySomething.CallOpts)
}

// GetUsers is a free data retrieval call binding the contract method 0x00ce8e3e.
//
// Solidity: function getUsers() view returns(address[])
func (_BuySomething *BuySomethingCallerSession) GetUsers() ([]common.Address, error) {
	return _BuySomething.Contract.GetUsers(&_BuySomething.CallOpts)
}

// GetUsersAmountByIndex is a free data retrieval call binding the contract method 0xadaf9e71.
//
// Solidity: function getUsersAmountByIndex(uint256 startIndex, uint256 endIndex) view returns(uint256[])
func (_BuySomething *BuySomethingCaller) GetUsersAmountByIndex(opts *bind.CallOpts, startIndex *big.Int, endIndex *big.Int) ([]*big.Int, error) {
	var out []interface{}
	err := _BuySomething.contract.Call(opts, &out, "getUsersAmountByIndex", startIndex, endIndex)

	if err != nil {
		return *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]*big.Int)).(*[]*big.Int)

	return out0, err

}

// GetUsersAmountByIndex is a free data retrieval call binding the contract method 0xadaf9e71.
//
// Solidity: function getUsersAmountByIndex(uint256 startIndex, uint256 endIndex) view returns(uint256[])
func (_BuySomething *BuySomethingSession) GetUsersAmountByIndex(startIndex *big.Int, endIndex *big.Int) ([]*big.Int, error) {
	return _BuySomething.Contract.GetUsersAmountByIndex(&_BuySomething.CallOpts, startIndex, endIndex)
}

// GetUsersAmountByIndex is a free data retrieval call binding the contract method 0xadaf9e71.
//
// Solidity: function getUsersAmountByIndex(uint256 startIndex, uint256 endIndex) view returns(uint256[])
func (_BuySomething *BuySomethingCallerSession) GetUsersAmountByIndex(startIndex *big.Int, endIndex *big.Int) ([]*big.Int, error) {
	return _BuySomething.Contract.GetUsersAmountByIndex(&_BuySomething.CallOpts, startIndex, endIndex)
}

// GetUsersByIndex is a free data retrieval call binding the contract method 0xfe36c56c.
//
// Solidity: function getUsersByIndex(uint256 startIndex, uint256 endIndex) view returns(address[])
func (_BuySomething *BuySomethingCaller) GetUsersByIndex(opts *bind.CallOpts, startIndex *big.Int, endIndex *big.Int) ([]common.Address, error) {
	var out []interface{}
	err := _BuySomething.contract.Call(opts, &out, "getUsersByIndex", startIndex, endIndex)

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetUsersByIndex is a free data retrieval call binding the contract method 0xfe36c56c.
//
// Solidity: function getUsersByIndex(uint256 startIndex, uint256 endIndex) view returns(address[])
func (_BuySomething *BuySomethingSession) GetUsersByIndex(startIndex *big.Int, endIndex *big.Int) ([]common.Address, error) {
	return _BuySomething.Contract.GetUsersByIndex(&_BuySomething.CallOpts, startIndex, endIndex)
}

// GetUsersByIndex is a free data retrieval call binding the contract method 0xfe36c56c.
//
// Solidity: function getUsersByIndex(uint256 startIndex, uint256 endIndex) view returns(address[])
func (_BuySomething *BuySomethingCallerSession) GetUsersByIndex(startIndex *big.Int, endIndex *big.Int) ([]common.Address, error) {
	return _BuySomething.Contract.GetUsersByIndex(&_BuySomething.CallOpts, startIndex, endIndex)
}

// Ids is a free data retrieval call binding the contract method 0xfac333ac.
//
// Solidity: function ids(uint256 ) view returns(uint256)
func (_BuySomething *BuySomethingCaller) Ids(opts *bind.CallOpts, arg0 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _BuySomething.contract.Call(opts, &out, "ids", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Ids is a free data retrieval call binding the contract method 0xfac333ac.
//
// Solidity: function ids(uint256 ) view returns(uint256)
func (_BuySomething *BuySomethingSession) Ids(arg0 *big.Int) (*big.Int, error) {
	return _BuySomething.Contract.Ids(&_BuySomething.CallOpts, arg0)
}

// Ids is a free data retrieval call binding the contract method 0xfac333ac.
//
// Solidity: function ids(uint256 ) view returns(uint256)
func (_BuySomething *BuySomethingCallerSession) Ids(arg0 *big.Int) (*big.Int, error) {
	return _BuySomething.Contract.Ids(&_BuySomething.CallOpts, arg0)
}

// Usdt is a free data retrieval call binding the contract method 0x2f48ab7d.
//
// Solidity: function usdt() view returns(address)
func (_BuySomething *BuySomethingCaller) Usdt(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _BuySomething.contract.Call(opts, &out, "usdt")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Usdt is a free data retrieval call binding the contract method 0x2f48ab7d.
//
// Solidity: function usdt() view returns(address)
func (_BuySomething *BuySomethingSession) Usdt() (common.Address, error) {
	return _BuySomething.Contract.Usdt(&_BuySomething.CallOpts)
}

// Usdt is a free data retrieval call binding the contract method 0x2f48ab7d.
//
// Solidity: function usdt() view returns(address)
func (_BuySomething *BuySomethingCallerSession) Usdt() (common.Address, error) {
	return _BuySomething.Contract.Usdt(&_BuySomething.CallOpts)
}

// Users is a free data retrieval call binding the contract method 0x365b98b2.
//
// Solidity: function users(uint256 ) view returns(address)
func (_BuySomething *BuySomethingCaller) Users(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _BuySomething.contract.Call(opts, &out, "users", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Users is a free data retrieval call binding the contract method 0x365b98b2.
//
// Solidity: function users(uint256 ) view returns(address)
func (_BuySomething *BuySomethingSession) Users(arg0 *big.Int) (common.Address, error) {
	return _BuySomething.Contract.Users(&_BuySomething.CallOpts, arg0)
}

// Users is a free data retrieval call binding the contract method 0x365b98b2.
//
// Solidity: function users(uint256 ) view returns(address)
func (_BuySomething *BuySomethingCallerSession) Users(arg0 *big.Int) (common.Address, error) {
	return _BuySomething.Contract.Users(&_BuySomething.CallOpts, arg0)
}

// UsersAmount is a free data retrieval call binding the contract method 0x0963b51e.
//
// Solidity: function usersAmount(uint256 ) view returns(uint256)
func (_BuySomething *BuySomethingCaller) UsersAmount(opts *bind.CallOpts, arg0 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _BuySomething.contract.Call(opts, &out, "usersAmount", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// UsersAmount is a free data retrieval call binding the contract method 0x0963b51e.
//
// Solidity: function usersAmount(uint256 ) view returns(uint256)
func (_BuySomething *BuySomethingSession) UsersAmount(arg0 *big.Int) (*big.Int, error) {
	return _BuySomething.Contract.UsersAmount(&_BuySomething.CallOpts, arg0)
}

// UsersAmount is a free data retrieval call binding the contract method 0x0963b51e.
//
// Solidity: function usersAmount(uint256 ) view returns(uint256)
func (_BuySomething *BuySomethingCallerSession) UsersAmount(arg0 *big.Int) (*big.Int, error) {
	return _BuySomething.Contract.UsersAmount(&_BuySomething.CallOpts, arg0)
}

// Buy is a paid mutator transaction binding the contract method 0xd6febde8.
//
// Solidity: function buy(uint256 num, uint256 id) returns()
func (_BuySomething *BuySomethingTransactor) Buy(opts *bind.TransactOpts, num *big.Int, id *big.Int) (*types.Transaction, error) {
	return _BuySomething.contract.Transact(opts, "buy", num, id)
}

// Buy is a paid mutator transaction binding the contract method 0xd6febde8.
//
// Solidity: function buy(uint256 num, uint256 id) returns()
func (_BuySomething *BuySomethingSession) Buy(num *big.Int, id *big.Int) (*types.Transaction, error) {
	return _BuySomething.Contract.Buy(&_BuySomething.TransactOpts, num, id)
}

// Buy is a paid mutator transaction binding the contract method 0xd6febde8.
//
// Solidity: function buy(uint256 num, uint256 id) returns()
func (_BuySomething *BuySomethingTransactorSession) Buy(num *big.Int, id *big.Int) (*types.Transaction, error) {
	return _BuySomething.Contract.Buy(&_BuySomething.TransactOpts, num, id)
}
