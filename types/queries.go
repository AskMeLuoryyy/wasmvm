package types

import (
	"encoding/json"
	"fmt"
)

//-------- Queries --------

// QueryResult is the Go counterpart of `ContractResult<Binary>`.
// The JSON annotations are used for deserializing directly. There is a custom serializer below.
type QueryResult queryResultImpl

type queryResultImpl struct {
	Ok  []byte `json:"ok,omitempty"`
	Err string `json:"error,omitempty"`
}

// A custom serializer that allows us to map QueryResult instances to the Rust
// enum `ContractResult<Binary>`
func (q QueryResult) MarshalJSON() ([]byte, error) {
	// In case both Ok and Err are empty, this is interpreted and serialized
	// as an Ok case with no data because errors must not be empty.
	if len(q.Ok) == 0 && len(q.Err) == 0 {
		return []byte(`{"ok":""}`), nil
	}
	return json.Marshal(queryResultImpl(q))
}

//-------- Querier -----------

// Querier is a thing that allows the contract to query information
// from the environment it is executed in. This is typically used to query
// a different contract or another module in a Cosmos blockchain.
//
// Queries are performed synchronously, i.e. the original caller is blocked
// until the query response is returned.
type Querier interface {
	// Query takes a query request, performs the query and returns the response.
	// It takes a gas limit measured in [CosmWasm gas] (aka. wasmvm gas) to ensure
	// the query does not consume more gas than the contract execution has left.
	//
	// [CosmWasm gas]: https://github.com/CosmWasm/cosmwasm/blob/v1.3.1/docs/GAS.md
	Query(request QueryRequest, gasLimit uint64) ([]byte, error)
	// GasConsumed returns the gas that was consumed by the querier during its entire
	// lifetime or by the context in which it was executed in. The absolute gas values
	// must not be used directly as it is undefined what is included in this value. Instead
	// wasmvm will call GasConsumed before and after the query and use the difference
	// as the query's gas usage.
	// Like the gas limit above, this is measured in [CosmWasm gas] (aka. wasmvm gas).
	//
	// [CosmWasm gas]: https://github.com/CosmWasm/cosmwasm/blob/v1.3.1/docs/GAS.md
	GasConsumed() uint64
}

// this is a thin wrapper around the desired Go API to give us types closer to Rust FFI
func RustQuery(querier Querier, binRequest []byte, gasLimit uint64) QuerierResult {
	var request QueryRequest
	err := json.Unmarshal(binRequest, &request)
	if err != nil {
		return QuerierResult{
			Err: &SystemError{
				InvalidRequest: &InvalidRequest{
					Err:     err.Error(),
					Request: binRequest,
				},
			},
		}
	}
	bz, err := querier.Query(request, gasLimit)
	return ToQuerierResult(bz, err)
}

// This is a 2-level result
type QuerierResult struct {
	Ok  *QueryResult `json:"ok,omitempty"`
	Err *SystemError `json:"error,omitempty"`
}

func ToQuerierResult(response []byte, err error) QuerierResult {
	if err == nil {
		return QuerierResult{
			Ok: &QueryResult{
				Ok: response,
			},
		}
	}
	syserr := ToSystemError(err)
	if syserr != nil {
		return QuerierResult{
			Err: syserr,
		}
	}
	return QuerierResult{
		Ok: &QueryResult{
			Err: err.Error(),
		},
	}
}

// QueryRequest is an rust enum and only (exactly) one of the fields should be set
// Should we do a cleaner approach in Go? (type/data?)
type QueryRequest struct {
	Bank         *BankQuery         `json:"bank,omitempty"`
	Custom       json.RawMessage    `json:"custom,omitempty"`
	IBC          *IBCQuery          `json:"ibc,omitempty"`
	Staking      *StakingQuery      `json:"staking,omitempty"`
	Distribution *DistributionQuery `json:"distribution,omitempty"`
	Stargate     *StargateQuery     `json:"stargate,omitempty"`
	Grpc         *GrpcQuery         `json:"grpc,omitempty"`
	Wasm         *WasmQuery         `json:"wasm,omitempty"`
}

type BankQuery struct {
	Supply           *SupplyQuery           `json:"supply,omitempty"`
	Balance          *BalanceQuery          `json:"balance,omitempty"`
	AllBalances      *AllBalancesQuery      `json:"all_balances,omitempty"`
	DenomMetadata    *DenomMetadataQuery    `json:"denom_metadata,omitempty"`
	AllDenomMetadata *AllDenomMetadataQuery `json:"all_denom_metadata,omitempty"`
}

type SupplyQuery struct {
	Denom string `json:"denom"`
}

// SupplyResponse is the expected response to SupplyQuery
type SupplyResponse struct {
	Amount Coin `json:"amount"`
}

type BalanceQuery struct {
	Address string `json:"address"`
	Denom   string `json:"denom"`
}

// BalanceResponse is the expected response to BalanceQuery
type BalanceResponse struct {
	Amount Coin `json:"amount"`
}

type AllBalancesQuery struct {
	Address string `json:"address"`
}

// AllBalancesResponse is the expected response to AllBalancesQuery
type AllBalancesResponse struct {
	Amount Array[Coin] `json:"amount"`
}

type DenomMetadataQuery struct {
	Denom string `json:"denom"`
}

type DenomMetadataResponse struct {
	Metadata DenomMetadata `json:"metadata"`
}

type AllDenomMetadataQuery struct {
	// Pagination is an optional argument.
	// Default pagination will be used if this is omitted
	Pagination *PageRequest `json:"pagination,omitempty"`
}

type AllDenomMetadataResponse struct {
	Metadata []DenomMetadata `json:"metadata"`
	// NextKey is the key to be passed to PageRequest.key to
	// query the next page most efficiently. It will be empty if
	// there are no more results.
	NextKey []byte `json:"next_key,omitempty"`
}

// IBCQuery defines a query request from the contract into the chain.
// This is the counterpart of [IbcQuery](https://github.com/CosmWasm/cosmwasm/blob/v0.14.0-beta1/packages/std/src/ibc.rs#L61-L83).
type IBCQuery struct {
	PortID       *PortIDQuery       `json:"port_id,omitempty"`
	ListChannels *ListChannelsQuery `json:"list_channels,omitempty"`
	Channel      *ChannelQuery      `json:"channel,omitempty"`
}

type PortIDQuery struct{}

type PortIDResponse struct {
	PortID string `json:"port_id"`
}

// ListChannelsQuery is an IBCQuery that lists all channels that are bound to a given port.
// If `PortID` is unset, this list all channels bound to the contract's port.
// Returns a `ListChannelsResponse`.
// This is the counterpart of [IbcQuery::ListChannels](https://github.com/CosmWasm/cosmwasm/blob/v0.14.0-beta1/packages/std/src/ibc.rs#L70-L73).
type ListChannelsQuery struct {
	// optional argument
	PortID string `json:"port_id,omitempty"`
}

type ListChannelsResponse struct {
	Channels Array[IBCChannel] `json:"channels"`
}

type ChannelQuery struct {
	// optional argument
	PortID    string `json:"port_id,omitempty"`
	ChannelID string `json:"channel_id"`
}

type ChannelResponse struct {
	// may be empty if there is no matching channel
	Channel *IBCChannel `json:"channel,omitempty"`
}

type StakingQuery struct {
	AllValidators  *AllValidatorsQuery  `json:"all_validators,omitempty"`
	Validator      *ValidatorQuery      `json:"validator,omitempty"`
	AllDelegations *AllDelegationsQuery `json:"all_delegations,omitempty"`
	Delegation     *DelegationQuery     `json:"delegation,omitempty"`
	BondedDenom    *struct{}            `json:"bonded_denom,omitempty"`
}

type AllValidatorsQuery struct{}

// AllValidatorsResponse is the expected response to AllValidatorsQuery
type AllValidatorsResponse struct {
	Validators Array[Validator] `json:"validators"`
}

type ValidatorQuery struct {
	/// Address is the validator's address (e.g. cosmosvaloper1...)
	Address string `json:"address"`
}

// ValidatorResponse is the expected response to ValidatorQuery
type ValidatorResponse struct {
	Validator *Validator `json:"validator"` // serializes to `null` when unset which matches Rust's Option::None serialization
}

type Validator struct {
	Address string `json:"address"`
	// decimal string, eg "0.02"
	Commission string `json:"commission"`
	// decimal string, eg "0.02"
	MaxCommission string `json:"max_commission"`
	// decimal string, eg "0.02"
	MaxChangeRate string `json:"max_change_rate"`
}

type AllDelegationsQuery struct {
	Delegator string `json:"delegator"`
}

type DelegationQuery struct {
	Delegator string `json:"delegator"`
	Validator string `json:"validator"`
}

// AllDelegationsResponse is the expected response to AllDelegationsQuery
type AllDelegationsResponse struct {
	Delegations Array[Delegation] `json:"delegations"`
}

type Delegation struct {
	Delegator string `json:"delegator"`
	Validator string `json:"validator"`
	Amount    Coin   `json:"amount"`
}

type DistributionQuery struct {
	// See <https://github.com/cosmos/cosmos-sdk/blob/c74e2887b0b73e81d48c2f33e6b1020090089ee0/proto/cosmos/distribution/v1beta1/query.proto#L222-L230>
	DelegatorWithdrawAddress *DelegatorWithdrawAddressQuery `json:"delegator_withdraw_address,omitempty"`
	// See <https://github.com/cosmos/cosmos-sdk/blob/c74e2887b0b73e81d48c2f33e6b1020090089ee0/proto/cosmos/distribution/v1beta1/query.proto#L157-L167>
	DelegationRewards *DelegationRewardsQuery `json:"delegation_rewards,omitempty"`
	// See <https://github.com/cosmos/cosmos-sdk/blob/c74e2887b0b73e81d48c2f33e6b1020090089ee0/proto/cosmos/distribution/v1beta1/query.proto#L180-L187>
	DelegationTotalRewards *DelegationTotalRewardsQuery `json:"delegation_total_rewards,omitempty"`
	// See <https://github.com/cosmos/cosmos-sdk/blob/b0acf60e6c39f7ab023841841fc0b751a12c13ff/proto/cosmos/distribution/v1beta1/query.proto#L202-L210>
	DelegatorValidators *DelegatorValidatorsQuery `json:"delegator_validators,omitempty"`
}

type DelegatorWithdrawAddressQuery struct {
	DelegatorAddress string `json:"delegator_address"`
}

type DelegatorWithdrawAddressResponse struct {
	WithdrawAddress string `json:"withdraw_address"`
}

type DelegationRewardsQuery struct {
	DelegatorAddress string `json:"delegator_address"`
	ValidatorAddress string `json:"validator_address"`
}

// See <https://github.com/cosmos/cosmos-sdk/blob/c74e2887b0b73e81d48c2f33e6b1020090089ee0/proto/cosmos/distribution/v1beta1/query.proto#L169-L178>
type DelegationRewardsResponse struct {
	Rewards []DecCoin `json:"rewards"`
}

type DelegationTotalRewardsQuery struct {
	DelegatorAddress string `json:"delegator_address"`
}

// See <https://github.com/cosmos/cosmos-sdk/blob/c74e2887b0b73e81d48c2f33e6b1020090089ee0/proto/cosmos/distribution/v1beta1/query.proto#L189-L200>
type DelegationTotalRewardsResponse struct {
	Rewards []DelegatorReward `json:"rewards"`
	Total   []DecCoin         `json:"total"`
}

type DelegatorReward struct {
	Reward           []DecCoin `json:"reward"`
	ValidatorAddress string    `json:"validator_address"`
}

type DelegatorValidatorsQuery struct {
	DelegatorAddress string `json:"delegator_address"`
}

// See <https://github.com/cosmos/cosmos-sdk/blob/b0acf60e6c39f7ab023841841fc0b751a12c13ff/proto/cosmos/distribution/v1beta1/query.proto#L212-L220>
type DelegatorValidatorsResponse struct {
	Validators []string `json:"validators"`
}

// DelegationResponse is the expected response to Array[Delegation]Query
type DelegationResponse struct {
	Delegation *FullDelegation `json:"delegation,omitempty"`
}

type FullDelegation struct {
	Delegator          string      `json:"delegator"`
	Validator          string      `json:"validator"`
	Amount             Coin        `json:"amount"`
	AccumulatedRewards Array[Coin] `json:"accumulated_rewards"`
	CanRedelegate      Coin        `json:"can_redelegate"`
}

type BondedDenomResponse struct {
	Denom string `json:"denom"`
}

// StargateQuery is encoded the same way as abci_query, with path and protobuf encoded request data.
// The format is defined in [ADR-21](https://github.com/cosmos/cosmos-sdk/blob/master/docs/architecture/adr-021-protobuf-query-encoding.md).
// The response is supposed to always be protobuf encoded data, but is JSON encoded on some chains.
// The caller is responsible for compiling the proper type definitions for both requests and responses.
type StargateQuery struct {
	// The expected protobuf message type (not [Any](https://protobuf.dev/programming-guides/proto3/#any)), binary encoded
	Data []byte `json:"data"`
	// The fully qualified endpoint path used for routing.
	// It follows the format `/service_path/method_name`,
	// eg. "/cosmos.authz.v1beta1.Query/Grants"
	Path string `json:"path"`
}

// GrpcQuery queries the chain using a grpc query.
// This allows to query information that is not exposed in our API.
// The chain needs to allowlist the supported queries.
//
// The returned data is protobuf encoded. The protobuf type depends on the query.
type GrpcQuery struct {
	// The expected protobuf message type (not [Any](https://protobuf.dev/programming-guides/proto3/#any)), binary encoded
	Data []byte `json:"data"`
	// The fully qualified endpoint path used for routing.
	// It follows the format `/service_path/method_name`,
	// eg. "/cosmos.authz.v1beta1.Query/Grants"
	Path string `json:"path"`
}

type WasmQuery struct {
	Smart        *SmartQuery        `json:"smart,omitempty"`
	Raw          *RawQuery          `json:"raw,omitempty"`
	ContractInfo *ContractInfoQuery `json:"contract_info,omitempty"`
	CodeInfo     *CodeInfoQuery     `json:"code_info,omitempty"`
	RawRange     *RawRangeQuery     `json:"raw_range,omitempty"`
}

// SmartQuery response is raw bytes ([]byte)
type SmartQuery struct {
	// Bech32 encoded sdk.AccAddress of the contract
	ContractAddr string `json:"contract_addr"`
	Msg          []byte `json:"msg"`
}

// RawQuery response is raw bytes ([]byte)
type RawQuery struct {
	// Bech32 encoded sdk.AccAddress of the contract
	ContractAddr string `json:"contract_addr"`
	Key          []byte `json:"key"`
}

type RawRangeQuery struct {
	// The address of the contract to query
	ContractAddr string `json:"contract_addr"`
	// Inclusive start bound. This is the first key you would like to get data for.
	//
	// If `start` is lexicographically greater than or equal to `end`, an empty range is described, mo matter of the order.
	Start []byte `json:"start,omitempty"`
	// Exclusive end bound. This is the key after the last key you would like to get data for.
	End []byte `json:"end,omitempty"`
	// Maximum number of elements to return.
	//
	// Make sure to set a reasonable limit to avoid running out of memory or into the deserialization limits of the VM. Also keep in mind that these limitations depend on the full JSON size of the response type.
	Limit uint16 `json:"limit"`
	// The order in which you want to receive the key-value pairs.
	Order string `json:"order"`
}

type RawRangeResponse struct {
	// The key-value pairs
	Data Array[RawRangeEntry] `json:"data"`
	// `None` if there are no more key-value pairs within the given key range.
	NextKey []byte `json:"next_key"`
}

type RawRangeEntry struct {
	Key   []byte
	Value []byte
}

func (r RawRangeEntry) MarshalJSON() ([]byte, error) {
	// marshal as [][]byte with two elements to match cosmwasm-std's RawRangeEntry
	return json.Marshal([][]byte{r.Key, r.Value})
}

func (r *RawRangeEntry) UnmarshalJSON(data []byte) error {
	// unmarshal as [][]byte with two elements
	var arr [][]byte
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	if len(arr) != 2 {
		return fmt.Errorf("invalid RawRange entry: expected array of length 2, got %d", len(arr))
	}
	r.Key = arr[0]
	r.Value = arr[1]
	return nil
}

type ContractInfoQuery struct {
	// Bech32 encoded sdk.AccAddress of the contract
	ContractAddr string `json:"contract_addr"`
}

type ContractInfoResponse struct {
	CodeID  uint64 `json:"code_id"`
	Creator string `json:"creator"`
	// Set to the admin who can migrate contract, if any
	Admin  string `json:"admin,omitempty"`
	Pinned bool   `json:"pinned"`
	// Set if the contract is IBC enabled
	IBCPort string `json:"ibc_port,omitempty"`
	// Set if the contract is IBC2 enabled
	IBC2Port string `json:"ibc2_port,omitempty"`
}

type CodeInfoQuery struct {
	CodeID uint64 `json:"code_id"`
}

type CodeInfoResponse struct {
	CodeID  uint64 `json:"code_id"`
	Creator string `json:"creator"`
	// Checksum is the hash of the Wasm blob. This field must always be set to a 32 byte value.
	// Everything else is considered a bug.
	Checksum Checksum `json:"checksum"`
}
