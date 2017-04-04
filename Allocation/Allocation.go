/*/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package main

import (
	"errors"
	"fmt"
	"strconv"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"math"
	"time"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/util"
)

type ManageAllocations struct {
}

type Transactions struct{
	TransactionId string `json:"transactionId"`
	TransactionDate string `json:"transactionDate"`
	DealID string `json:"dealId"`
	Pledger string `json:"pledger"`
	Pledgee string `json:"pledgee"`
	RQV string `json:"rqv"`
	Currency string `json:"currency"`
	CurrencyConversionRate string `json:"currencyConversionRate"`
	MarginCAllDate string `json:"marginCAllDate"`
	AllocationStatus string `json:"allocationStatus"`
	TransactionStatus string `json:"transactionStatus"`
}

type Deals struct{							// Attributes of a Allocation
	DealID string `json:"dealId"`
	Pledger string `json:"pledger"`
	Pledgee string `json:"pledgee"`
	MaxValue string `json:"maxValue"`		//Maximum Value of all the securities of each Collateral Form 
	TotalValueLongBoxAccount string `json:"totalValueLongBoxAccount"`
	TotalValueSegregatedAccount string `json:"totalValueSegregatedAccount"`
	IssueDate string `json:"issueDate"`
	LastSuccessfulAllocationDate string `json:"lastSuccessfulAllocationDate"`
	Transactions string `json:"transactions"`
}

type Accounts struct{
	AccountID string `json:"accountId"`
	AccountName string `json:"accountName"`
	AccountNumber string `json:"accountNumber"`
	AccountType string `json:"accountType"`
	TotalValue string `json:"totalValue"`
	Currency string `json:"currency"`
	Pledger string `json:"pledger"`
	Securities string `json:"securities"`
}

type Securities struct{
	SecurityId string `json:"securityId"`
	AccountNumber string `json:"accountNumber"`
	SecuritiesName string `json:"securityName"`
	SecuritiesQuantity string `json:"securityQuantity"`
	SecurityType string `json:"securityType"`
	CollateralForm string `json:"collateralForm"`
	TotalValue string `json:"totalValue"`
	ValuePercentage string `json:"valuePercentage"`
	MTM string `json:"mtm"`
	EffectivePercentage string `json:"effectivePercentage"`
	EffectiveValueinUSD string `json:"effectiveValueinUSD"`
	Currency string `json:"currency"`
}

// Used for Security Array Sort
// Reference at https://play.golang.org/p/Rz9NCEVhGu
type SecurityArrayStruct []Securities

func (slice SecurityArrayStruct) Len() int {	return len(slice) }
func (slice SecurityArrayStruct) Less(i, j int) bool {	// Sorting through the field 'ValuePercentage' for now as it contians the Priority
	return slice[i].ValuePercentage < slice[j].ValuePercentage;}
func (slice SecurityArrayStruct) Swap(i, j int) {	slice[i], slice[j] = slice[j], slice[i] }

// Use as Object.Security["CommonStocks"][0]
// Reference [Tested by Pranav] https://play.golang.org/p/JlQJF5Z14X
type Ruleset struct{
	Security map[string][]float64 `json:"Security"`
	BaseCurrency string `json:"BaseCurrency"`
	EligibleCurrency []string `json:"EligibleCurrency"`
}

// Use as Object.Rates["EUR"]
// Reference [Tested by Pranav] https://play.golang.org/p/j5Act-jN5C
type CurrencyConversion struct{
	Base string `json:"base"`
	Date string `json:"date"`
	Rates map[string ]float64 `json:"rates"`
}

// To be used as SecurityJSON["CommonStocks"]["Priority"] ==> 1
 var SecurityJSON = map[string]map[string]string{ 
					"CommonStocks"				: map[string]string{ "ConcentrationLimit" : "40" ,	"Priority" : "1" ,	"ValuationPercentage" : "97" } ,
					"CorporateBonds"			: map[string]string{ "ConcentrationLimit" : "30" ,	"Priority" : "2" ,	"ValuationPercentage" : "97" } ,
					"SovereignBonds"			: map[string]string{ "ConcentrationLimit" : "25" ,	"Priority" : "3" ,	"ValuationPercentage" : "95" } ,
					"USTreasuryBills"			: map[string]string{ "ConcentrationLimit" : "25" ,	"Priority" : "4" ,	"ValuationPercentage" : "95" } ,
					"USTreasuryBonds"			: map[string]string{ "ConcentrationLimit" : "25" ,	"Priority" : "5" ,	"ValuationPercentage" : "95" } ,
					"USTreasuryNotes"			: map[string]string{ "ConcentrationLimit" : "25" ,	"Priority" : "6" ,	"ValuationPercentage" : "95" } ,
					"Gilt"						: map[string]string{ "ConcentrationLimit" : "25" ,	"Priority" : "7" ,	"ValuationPercentage" : "94" } ,
					"FederalAgencyBonds"		: map[string]string{ "ConcentrationLimit" : "20" ,	"Priority" : "8" ,	"ValuationPercentage" : "93" } ,
					"GlobalBonds"				: map[string]string{ "ConcentrationLimit" : "20" ,	"Priority" : "9" ,	"ValuationPercentage" : "92" } ,
					"PreferrredShares"			: map[string]string{ "ConcentrationLimit" : "20" ,	"Priority" : "10",	"ValuationPercentage" : "91" } ,
					"ConvertibleBonds"			: map[string]string{ "ConcentrationLimit" : "20" ,	"Priority" : "11",	"ValuationPercentage" : "90" } ,
					"RevenueBonds"				: map[string]string{ "ConcentrationLimit" : "15" ,	"Priority" : "12",	"ValuationPercentage" : "90" } ,
					"MediumTermNote"			: map[string]string{ "ConcentrationLimit" : "15" ,	"Priority" : "13",	"ValuationPercentage" : "89" } ,
					"ShortTermInvestments"		: map[string]string{ "ConcentrationLimit" : "15" ,	"Priority" : "14",	"ValuationPercentage" : "87" } ,
					"BuilderBonds"				: map[string]string{ "ConcentrationLimit" : "15" ,	"Priority" : "15",	"ValuationPercentage" : "85" }}


// ============================================================================================================================
// Main - start the chaincode for Allocation management
// ============================================================================================================================
func main() {			
	err := shim.Start(new(ManageAllocations))
	if err != nil {
		fmt.Printf("Error starting Allocation management chaincode: %s", err)
	}
}
// ============================================================================================================================
// Init - reset all the things
// ============================================================================================================================
func (t *ManageAllocations) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	var msg string
	var err error
	if len(args) != 1 {
		errMsg := "{ \"message\" : \"Incorrect number of arguments. Expecting ' ' as an argument\", \"code\" : \"503\"}"
		err = stub.SetEvent("errEvent", []byte(errMsg))
		if err != nil {
			return nil, err
		} 
		return nil, nil
	}
	// Initialize the chaincode
	msg = args[0]
	// Write the state to the ledger
	err = stub.PutState("abc", []byte(msg))				//making a test var "abc", I find it handy to read/write to it right away to test the network
	if err != nil {
		return nil, err
	}
	var empty []string
	jsonAsBytes, _ := json.Marshal(empty)								//marshal an emtpy array of strings to clear the index
	err = stub.PutState("_init", jsonAsBytes)
	if err != nil {
		return nil, err
	}

	tosend := "{ \"message\" : \"ManageAllocations chaincode is deployed successfully.\", \"code\" : \"200\"}"
	err = stub.SetEvent("evtsender", []byte(tosend))
	if err != nil {
		return nil, err
	} 
	return nil, nil
}
// ============================================================================================================================
// Run - Our entry Dealint for Invocations - [LEGACY] obc-peer 4/25/2016
// ============================================================================================================================
func (t *ManageAllocations) Run(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	fmt.Println("run is running " + function)
	return t.Invoke(stub, function, args)
}
// ============================================================================================================================
// Invoke - Our entry Dealint for Invocations
// ============================================================================================================================
func (t *ManageAllocations) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	fmt.Println("invoke is running " + function)

	// Handle different functions
	if function == "init" {													// Initialize the chaincode state, used as reset
		return t.Init(stub, "init", args)
	}else if function == "start_allocation" {								// Create a new Allocation
		return t.start_allocation(stub, args)
	}else if function == "LongboxAccountUpdated" {							// Secondary Fire when Longbox account is updated
		return t.LongboxAccountUpdated(stub, args)
	}
	fmt.Println("invoke did not find func: " + function)
	errMsg := "{ \"message\" : \"Received unknown function invocation\", \"code\" : \"503\"}"
	err := stub.SetEvent("errEvent", []byte(errMsg))
	if err != nil {
		return nil, err
	} 
	return nil, nil			
}
// ============================================================================================================================
// Query - Our entry Dealint for Queries
// ============================================================================================================================

func (t *ManageAllocations) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	fmt.Println("query is running " + function)

	// Handle different functions
	/*if function == "nil" {									
		return t.nil(stub, args)
	}*/
	fmt.Println("Allocation does not support query functions.")	
	errMsg := "{ \"message\" : \"Allocation does not support query functions.\", \"code\" : \"503\"}"
	err := stub.SetEvent("errEvent", []byte(errMsg))
	if err != nil {
		return nil, err
	} 
	return nil, nil
}
// ============================================================================================================================
// A used updated his :LongBox Account - create a new Allocation, store into chaincode state
// ============================================================================================================================
func (t *ManageAllocations) LongboxAccountUpdated(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	var err error
	if len(args) != 4 {
		errMsg := "{ \"message\" : \"Incorrect number of arguments. Expecting 4\", \"code\" : \"503\"}"
		err = stub.SetEvent("errEvent", []byte(errMsg))
		if err != nil {
			return nil, err
		} 
		return nil, nil
	}
	fmt.Println("start LongboxAccountUpdated")

	_DealChaincode	:= args[0]
	_AccountName	:= args[1]
	_AccountType	:= args[2]
	_CurrentTimeStamp	:= args[3]

	var TransactionsDataFetched []Transactions

	// Fetching Attl transactions for the user
	function := "getTransactions_byUser"
	invokeArgs := util.ToChaincodeArgs(function, _AccountName, _AccountType)
	result, err := stub.InvokeChaincode(_DealChaincode, invokeArgs)
	if err != nil {
		errStr := fmt.Sprintf("Error in fetching Transactions from 'Deal' chaincode. Got error: %s", err.Error())
		fmt.Printf(errStr)
		return nil, errors.New(errStr)
	}
	json.Unmarshal(result, &TransactionsDataFetched)

	// Timestamp to Date/Time Objest in Go and Logic behind cutoff time
	// Ref: https://play.golang.org/p/KJRigmHzu9

	i, err := strconv.ParseInt(_CurrentTimeStamp , 10, 64)
	if err != nil {
		panic(err)
	}
	_CurrentTimeObj := time.Unix(i, 0)

	var newTxStatus, newAllStatus string

	for _, ValueTransaction := range TransactionsDataFetched {
		if ValueTransaction.TransactionStatus == "Pending" {

			i, err := strconv.ParseInt(ValueTransaction.MarginCAllDate , 10, 64)
			if err != nil {
				panic(err)
			}
			_MarginCallTimeObj := time.Unix(i, 0)


			if _CurrentTimeObj.Sub(_MarginCallTimeObj).Hours() <= 24 && _CurrentTimeObj.Sub(_MarginCallTimeObj).Hours() >= 0 { 	
				// New securites are uploaded in cutoff time
				newTxStatus 	= "Ready"
				newAllStatus 	= "Ready for Allocation"
			}else{
				// New securities not uploaded in cutoff time
				newTxStatus 	= "Failed"
				newAllStatus 	= "Allocation Failed"
			}
			
			order:= `[` +
				`"` + ValueTransaction.TransactionId + `" , ` + 
				`"` + ValueTransaction.TransactionDate + `" , ` + 
				`"` + ValueTransaction.DealID + `" , ` + 
				`"` + ValueTransaction.Pledger + `" , ` + 
				`"` + ValueTransaction.Pledgee + `" , ` + 
				`"` + ValueTransaction.RQV + `" , ` +
		        `"` + ValueTransaction.Currency + `" , ` + 
		        `"` + ValueTransaction.CurrencyConversionRate + `" , ` +  
		        `"` + ValueTransaction.MarginCAllDate + `" , ` + 
		        `"` + newAllStatus + `" , ` + 
		        `"` + newTxStatus + `" ` + 
		        `]`
		    // Update allocation status of a transaction
			function = "update_transaction"
			invokeArgs := util.ToChaincodeArgs(function, order)
			result, err := stub.InvokeChaincode(_DealChaincode, invokeArgs)
			if err != nil {
				errStr := fmt.Sprintf("Failed to update Transaction status from 'Deal' chaincode. Got error: %s", err.Error())
				fmt.Printf(errStr)
				return nil, errors.New(errStr)
			}
			fmt.Println("Transaction hash returned: ",result)
		    fmt.Println(ValueTransaction.TransactionId + " updated with AllocationStatus as " + newAllStatus) 
		    fmt.Println(ValueTransaction.TransactionId + " updated with TransactionStatus as " + newTxStatus) 

		    //Sending event call
		    tosend:= "{ \"transactionId\" : \"" + ValueTransaction.TransactionId + "\", \"message\" : \"Transaction updated succcessfully with AllocationStatus"		    			+ newAllStatus +" \", \"code\" : \"200\"}"
		    err = stub.SetEvent("evtsender", [] byte(tosend))
		    if err != nil {
		        return nil, err
		    }
		}
	}

	fmt.Println("end LongboxAccountUpdated")
	return nil,nil
}
// ============================================================================================================================
// Start Allocation - create a new Allocation, store into chaincode state
// ============================================================================================================================
func (t *ManageAllocations) start_allocation(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var err error
	if len(args) != 8 {
		errMsg := "{ \"message\" : \"Incorrect number of arguments. Expecting 8\", \"code\" : \"503\"}"
		err = stub.SetEvent("errEvent", []byte(errMsg))
		if err != nil {
			return nil, err
		} 
		return nil, nil
	}
	fmt.Println("start start_allocation")
	
	// Alloting Params
	DealChaincode							:= args[0]
	AccountChainCode 						:= args[1]
	APIIP									:= args[2]
	DealID 									:= args[3]
	TransactionID 							:= args[4]
	PledgerLongboxAccount					:= args[5]
	PledgeeSegregatedAccount				:= args[6]
	MarginCallTimpestamp					:= args[7]


	//-----------------------------------------------------------------------------

	// Fetch Deal details from Blockchain
	f := "getDeal_byID"
	queryArgs := util.ToChaincodeArgs(f, DealID)
	dealAsBytes, err := stub.QueryChaincode(DealChaincode, queryArgs)
	if err != nil {
		errStr := fmt.Sprintf("Failed to query chaincode. Got error: %s", err.Error())
		fmt.Printf(errStr)
		return nil, errors.New(errStr)
	} 	
	DealData := Deals{}
	json.Unmarshal(dealAsBytes, &DealData)
	fmt.Println(DealData);
	if DealData.DealID == DealID{
		fmt.Println("Deal found with DealID : " + DealID)
	}else{
		errMsg := "{ \"message\" : \""+ DealID+ " Not Found.\", \"code\" : \"503\"}"
		err = stub.SetEvent("errEvent", []byte(errMsg))
		if err != nil {
			return nil, err
		} 
		return nil, nil
	}

	Pledger := DealData.Pledger
	Pledgee := DealData.Pledgee
	fmt.Println("Pledger : " , Pledger)
	fmt.Println("Pledgee : " , Pledgee)

	// Fetch Transaction details from Blockchain
	function := "getTransaction_byID"
	queryArgs = util.ToChaincodeArgs(function, TransactionID)
	transactionAsBytes, err := stub.QueryChaincode(DealChaincode, queryArgs)
	if err != nil {
		errStr := fmt.Sprintf("Failed to query chaincode. Got error: %s", err.Error())
		fmt.Printf(errStr)
		return nil, errors.New(errStr)
	} 
	TransactionData := Transactions{}
	json.Unmarshal(transactionAsBytes, &TransactionData)
	fmt.Println(TransactionData);
	if TransactionData.TransactionId == TransactionID{
		fmt.Println("Transaction found with TransactionID : " + TransactionID)
	}else{
		errMsg := "{ \"message\" : \""+ TransactionID+ " Not Found.\", \"code\" : \"503\"}"
		err = stub.SetEvent("errEvent", []byte(errMsg))
		if err != nil {
			return nil, err
		} 
		return nil, nil
	}

	/*RQV,errBool := strconv.ParseFloat(TransactionData.RQV)*/
	RQV, errBool := strconv.ParseFloat(TransactionData.RQV,64)
	if errBool != nil { fmt.Println(errBool) }
	fmt.Println("RQV : " , RQV)

	//-----------------------------------------------------------------------------

	// Update allocation status to "Allocation in progress"
	function = "update_transaction_AllocationStatus"
	invokeArgs := util.ToChaincodeArgs(function, TransactionID, "Allocation in progress")
	result, err := stub.InvokeChaincode(DealChaincode, invokeArgs)
	if err != nil {
		errStr := fmt.Sprintf("Failed to update Transaction status from 'Deal' chaincode. Got error: %s", err.Error())
		fmt.Printf(errStr)
		return nil, errors.New(errStr)
	}
	fmt.Sprintf("Transaction hash returned: %s",result)
	fmt.Println("Successfully updated allocation status to 'Allocation in progress'")

	//-----------------------------------------------------------------------------

	// Fetching the Private Securtiy Ruleset based on Pledger & Pledgee
	// Escaping the values to be put in URL
	PledgerESC := url.QueryEscape(Pledger)
	PledgeeESC := url.QueryEscape(Pledgee)

	url := fmt.Sprintf("http://%s/securityRuleset/%s/%s", APIIP, PledgerESC, PledgeeESC)
	fmt.Println("URL for Ruleset : " + url)

	// Build the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("Ruleset fetch error: ", err)
		return nil, err
	}

	// For control over HTTP client headers, redirect policy, and other settings, create a Client
	// A Client is an HTTP client
	client := &http.Client{}

	// Send the request via a client 
	// Do sends an HTTP request and returns an HTTP response
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Do: ", err)
		errMsg := "{ \"message\" : \"Unable to fetch Security Ruleset at "+ APIIP +".\", \"code\" : \"503\"}"
		err = stub.SetEvent("errEvent", []byte(errMsg))
		if err != nil {
			return nil, err
		} 
	}

	fmt.Println("The SecurityRuleset response is::"+strconv.Itoa(resp.StatusCode))

	// Varaible record to be filled with the data from the JSON
	var rulesetFetched Ruleset

	// Use json.Decode for reading streams of JSON data and store it
	if err := json.NewDecoder(resp.Body).Decode(&rulesetFetched); 
	err != nil {
		fmt.Println(err)
	}

	// Callers should close resp.Body when done reading from it 
	// Defer the closing of the body
	defer resp.Body.Close()

	fmt.Println("Ruleset : ")
	fmt.Println(rulesetFetched)

	//-----------------------------------------------------------------------------
	
	/*	Fetching Currency coversion rates in bast form of USD.
		Sample Response as JSON:
		{
			"base": "USD",
			"date": "2017-03-20",
			"rates": {
				"AUD": 1.2948,
				"BGN": 1.819,
				"BRL": 3.1079,
				"CAD": 1.3355,
				"CHF": 0.99702,
				"CNY": 6.9074,
				"CZK": 25.131,
				"DKK": 6.9146,
				"GBP": 0.80723,
				"HKD": 7.7657,
				"HRK": 6.8876,
				"HUF": 287.05,
				"IDR": 13314,
				"ILS": 3.6313,
				"INR": 65.365,
				"JPY": 112.71,
				"KRW": 1115.7,
				"MXN": 19.114,
				"MYR": 4.4265,
				"NOK": 8.4894,
				"NZD": 1.4203,
				"PHP": 50.061,
				"PLN": 3.9825,
				"RON": 4.2415,
				"RUB": 57.53,
				"SEK": 8.8428,
				"SGD": 1.3979,
				"THB": 34.725,
				"TRY": 3.6335,
				"ZAR": 12.676,
				"EUR": 0.93006
			}
		}
	*/
	url2 := fmt.Sprintf("http://api.fixer.io/latest?base=USD")

	// Build the request
	req2, err2 := http.NewRequest("GET", url2, nil)
	if err2 != nil {
		fmt.Println("Currency coversion rate fetch error: ", err2)
		return nil, err2
	}

	// For control over HTTP client headers, redirect policy, and other settings, create a Client
	// A Client is an HTTP client
	client2 := &http.Client{}

	// Send the request via a client 
	// Do sends an HTTP request and returns an HTTP response
	resp2, err2 := client2.Do(req2)
	if err2 != nil {
		fmt.Println("Do: ", err2)
		errMsg := "{ \"message\" : \"Unable to fetch Currency Exchange Rates from: "+ url2 +".\", \"code\" : \"503\"}"
		err2 = stub.SetEvent("errEvent", []byte(errMsg))
		if err2 != nil {
			return nil, err2
		} 
	}

	fmt.Println("The SecurityRuleset response is::"+strconv.Itoa(resp2.StatusCode))

	// Varaible ConversionRate to be filled with the data from the JSON
	var ConversionRate CurrencyConversion

	// Use json.Decode for reading streams of JSON data and store it
	if err := json.NewDecoder(resp2.Body).Decode(&ConversionRate); 
	err != nil {
		fmt.Println(err)
	}

	// Callers should close resp.Body when done reading from it 
	// Defer the closing of the body
	defer resp2.Body.Close()


	fmt.Println("Exchange Rate : ")
	fmt.Println(ConversionRate)

	//-----------------------------------------------------------------------------

	// Caluculate eligible Collateral value from RQV
	RQVEligibleValue := make(map[string]float64)

	//Iterating through all the securities present in the ruleset
	for key,value := range rulesetFetched.Security {
		// key = "CommonStocks" && value = [35, 1, 95]
		// value[0] => ConcentrationLimit
		// value[1] => Priority
		// value[2] => ValuationPercentage

		PriorityPri := value[1]
		PriorityPub, errBool := strconv.ParseFloat(SecurityJSON[key]["Priority"],64)
		if errBool != nil { fmt.Println(errBool) }

		ConcentrationLimitPri := value[0]
		ConcentrationLimitPub, errBool := strconv.ParseFloat(SecurityJSON[key]["ConcentrationLimit"],64)
		if errBool != nil { fmt.Println(errBool) }

		ValuationPercentagePri := value[2]
		ValuationPercentagePub, errBool := strconv.ParseFloat(SecurityJSON[key]["ValuationPercentage"],64)
		if errBool != nil { fmt.Println(errBool) }

		// Check if privateset is subset of publicset
		if PriorityPub > PriorityPri && ConcentrationLimitPub > ConcentrationLimitPri && ValuationPercentagePub > ValuationPercentagePri {
			errMsg := "{ \"message\" : \"Security Ruleset out of allows values for: "+ key +".\", \"code\" : \"503\"}"
			err = stub.SetEvent("errEvent", []byte(errMsg))
			if err != nil {
				return nil, err
			} 
		} else {
			RQVtemp, errBool := strconv.ParseFloat(SecurityJSON[key]["Priority"],64)
			if errBool != nil { fmt.Println(errBool) }

			RQVEligibleValue[key] = float64(RQVtemp * ConcentrationLimitPri)
		}
	}
	fmt.Println("RQVEligibleValue after calculation:")
	fmt.Printf("%#v",RQVEligibleValue)


	//-----------------------------------------------------------------------------

	// Fetch Pledger & Pledgee securities for longbox and segregated accounts
	function = "getSecrurities_byAccount"
	
	queryArgs = util.ToChaincodeArgs(function, PledgerLongboxAccount)
	PledgerLongboxSecuritiesString, err := stub.QueryChaincode(AccountChainCode, queryArgs)

	queryArgs = util.ToChaincodeArgs(function, PledgeeSegregatedAccount)
	PledgeeSegregatedSecuritiesString, err := stub.QueryChaincode(AccountChainCode, queryArgs)

	/**	Calculate the effective value and total value of each Security present in the Longbox account of the pledger
		and the Segregated account of the pledgee
	*/
	var TotalValuePledgerLongbox, TotalValuePledgeeSegregated, AvailableEligibleCollateral  float64
	var PledgerLongboxSecurities,PledgeeSegregatedSecurities, CombinedSecurities []Securities

	// Make inteface to receive string. UnMarshal them extract them and make an array out of them.
	var PledgerLongboxSecuritiesJSON, PledgeeSegregatedSecuritiesJSON SecurityArrayStruct
	json.Unmarshal(PledgerLongboxSecuritiesString, &PledgerLongboxSecuritiesJSON)
	json.Unmarshal(PledgeeSegregatedSecuritiesString, &PledgeeSegregatedSecuritiesJSON)

	TotalValuePledgerLongboxSecurities := make(map[string]float64)
	TotalValuePledgeeSegregatedSecurities := make(map[string]float64)

	//Operations for Pledger Longbox Securities
	for _,value := range PledgerLongboxSecuritiesJSON {
		// Key = Security ID && value = Security Structure
		tempSecurity := Securities{}
		tempSecurity = value

		// Check if Current Collateral Form type is acceptied in ruleset. If not skip it!
		if len(rulesetFetched.Security[tempSecurity.CollateralForm]) > 0 {



			url2 := fmt.Sprintf("http://"+ APIIP +"/MarketData/" + tempSecurity.SecurityId)

			// Build the request
			req2, err2 := http.NewRequest("GET", url2, nil)
			if err2 != nil {
				fmt.Println("Market rate fetch error: ", err2)
				return nil, err2
			}

			// For control over HTTP client headers, redirect policy, and other settings, create a Client
			// A Client is an HTTP client
			client2 := &http.Client{}

			// Send the request via a client 
			// Do sends an HTTP request and returns an HTTP response
			resp2, err2 := client2.Do(req2)
			if err2 != nil {
				fmt.Println("Do: ", err2)
				errMsg := "{ \"message\" : \"Unable to fetch Market Rates from: "+ url2 +".\", \"code\" : \"503\"}"
				err2 = stub.SetEvent("errEvent", []byte(errMsg))
				if err2 != nil {
					return nil, err2
				}
			}

			fmt.Println("The MarketData response is::"+strconv.Itoa(resp2.StatusCode))


			var stringArr []string

			// Use json.Decode for reading streams of JSON data and store it
			if err := json.NewDecoder(resp2.Body).Decode(&stringArr); 
			err != nil {
				fmt.Println(err)
			}
			// Callers should close resp.Body when done reading from it 
			// Defer the closing of the body
			defer resp2.Body.Close()

			tempSecurity.MTM = stringArr[0]

			// Storing the Value percentage in the security ruleset data itself
			tempSecurity.EffectivePercentage = strconv.FormatFloat(rulesetFetched.Security[tempSecurity.CollateralForm][2], 'E', -1, 64)

			// Effective Value = Currency conversion rate(to USD) * MTM(market Value)
			temp, errBool := strconv.ParseFloat(tempSecurity.MTM,64)
			if errBool != nil { fmt.Println(errBool) }

			temp3 := ConversionRate.Rates[tempSecurity.Currency] * float64(temp)
			tempSecurity.EffectiveValueinUSD = strconv.FormatFloat(temp3, 'E', -1, 64)

			// Adding it to TotalValue
			temp, errBool = strconv.ParseFloat(tempSecurity.EffectiveValueinUSD,64)
			if errBool != nil { fmt.Println(errBool) }
			temp2, errBool := strconv.ParseFloat(tempSecurity.SecuritiesQuantity,64)
			if errBool != nil { fmt.Println(errBool) }

			tempTotal := temp * temp2
			tempSecurity.TotalValue = strconv.FormatFloat(tempTotal, 'E', -1, 64)

			// Calculate Total based on Security Type
			TotalValuePledgerLongboxSecurities[tempSecurity.CollateralForm] += float64(tempTotal)
			TotalValuePledgerLongbox += float64(tempTotal)
			AvailableEligibleCollateral += float64(tempTotal)

			/*	Warning :
				Saving Priority for the Security in filed `ValuePercentage`
				This is just for using the limited sorting application provided by GOlang
				By no chance is this to be stored on Blockchain. 
			*/
			tempSecurity.ValuePercentage = strconv.FormatFloat(rulesetFetched.Security[tempSecurity.CollateralForm][1], 'E', -1, 64)

			// Append Securities to an array
			PledgerLongboxSecurities = append(PledgerLongboxSecurities,tempSecurity)
			CombinedSecurities = append(CombinedSecurities,tempSecurity)
		}
	}

	// Operations for Pledgee Segregated Account(s)
	for _,value := range PledgeeSegregatedSecuritiesJSON {
		// Key = Security ID && value = Security Structure
		tempSecurity := Securities{}
		tempSecurity = value

		// Check if Current Collateral Form type is acceptied in ruleset. If not skip it!
		if len(rulesetFetched.Security[tempSecurity.CollateralForm]) > 0 {

			// Storing the Value percentage in the security data itself
			tempSecurity.EffectivePercentage = SecurityJSON[tempSecurity.SecurityId]["ValuePercentage"]

			// Effective Value = Currency conversion rate(to USD) * MTM(market Value)

			temp, errBool := strconv.ParseFloat(tempSecurity.MTM,64)
			if errBool != nil { fmt.Println(errBool) }

			temp3 := ConversionRate.Rates[tempSecurity.Currency] * float64(temp)
			tempSecurity.EffectiveValueinUSD = strconv.FormatFloat(temp3, 'E', -1, 64)

			// Adding it to TotalValue

			temp, errBool = strconv.ParseFloat(tempSecurity.EffectiveValueinUSD,64)
			if errBool != nil { fmt.Println(errBool) }
			temp2, errBool := strconv.ParseFloat(tempSecurity.SecuritiesQuantity,64)
			if errBool != nil { fmt.Println(errBool) }

			tempTotal := temp * temp2
			tempSecurity.TotalValue = strconv.FormatFloat(tempTotal, 'E', -1, 64)

			// Calculate Total based on Security Type
			TotalValuePledgeeSegregatedSecurities[tempSecurity.CollateralForm] += float64(tempTotal)
			TotalValuePledgeeSegregated += float64(tempTotal)
			AvailableEligibleCollateral =+ float64(tempTotal)

			/*	Warning :
				Saving Priority for the Security in filed `ValuePercentage`
				This is just for using the limited sorting application provided by GOlang
				By no chance is this to be stored on Blockchain. 
			*/
			tempSecurity.ValuePercentage = strconv.FormatFloat(rulesetFetched.Security[tempSecurity.CollateralForm][1], 'E', -1, 64)

			// Append Securities to an array
			PledgeeSegregatedSecurities = append(PledgeeSegregatedSecurities, tempSecurity)
			CombinedSecurities = append(CombinedSecurities,tempSecurity)
		}

	}


	//-----------------------------------------------------------------------------

	if AvailableEligibleCollateral < float64(RQV) {
		// Value of eligible collateral available in Pledger Long acc + Pledgee Seg acc < RQV 

			// Update the margin call s Allocation Status as Pending due to insufficient collateral
			/*function := "update_transaction_AllocationStatus"
			invokeArgs := util.ToChaincodeArgs(function, TransactionID, "Pending due to insufficient collateral")
			result, err := stub.InvokeChaincode(DealChaincode, invokeArgs)
			if err != nil {
				errStr := fmt.Sprintf("Failed to update Transaction status from 'Deal' chaincode. Got error: %s", err.Error())
				fmt.Printf(errStr)
				return nil, errors.New(errStr)
			}*/

		// Update transaction's allocation status to "Pending due to insufficient collateral" and transaction status to "Pending"
		f := "update_transaction"
		input:= `[` +
			TransactionData.TransactionId + `" , ` + 
			TransactionData.TransactionDate + `" ,`+ 
			TransactionData.DealID + `" , ` + 
			TransactionData.Pledger + `" , ` + 
			TransactionData.Pledgee + `" , ` + 
			TransactionData.RQV + `" , ` +
	        TransactionData.Currency + `" , ` + 
	        "" + `" , ` +  
	        MarginCallTimpestamp + `" , ` + 
	        "Pending due to insufficient collateral" + `" , ` + 
	        "Pending" + `" ` + 
	        `]`
	    fmt.Println(input);
		invoke_args := util.ToChaincodeArgs(f, input)
		result, err := stub.InvokeChaincode(DealChaincode, invoke_args)
		if err != nil {
			errStr := fmt.Sprintf("Failed to invoke chaincode. Got error: %s", err.Error())
			fmt.Printf(errStr)
			return nil, errors.New(errStr)
		} 	
		fmt.Printf("Update transaction returned : %s",result)
		fmt.Println("Successfully updated allocation status to 'Pending due to insufficient collateral'")
	    //Send a event to event handler
	    tosend:= "{ \"transactionId\" : \"" + TransactionData.TransactionId + "\", \"message\" : \"Transaction updated succcessfully with status \"Pending\"\", \"code\" : \"200\"}"
	    err = stub.SetEvent("evtsender", [] byte(tosend))
	    if err != nil {
	        return nil, err
	    }

	    // Actual return of process end. 
		ret:= "{ \"message\" : \""+TransactionID+" pending due to insufficient collateral. Notification sent to user.\", \"code\" : \"200\"}"
		return []byte(ret), nil

	} else {

		// Value of eligible collateral available in Pledger Long acc + Pledgee Seg acc >= RQV 

		//-----------------------------------------------------------------------------

		// Sorting the Securities in PledgerLongboxSecurities & PledgeeSegregatedSecurities
		// Using Code defination like https://play.golang.org/p/ciN45THQjM
		// Reference from http://nerdyworm.com/blog/2013/05/15/sorting-a-slice-of-structs-in-go/
		
		sort.Sort(SecurityArrayStruct(PledgerLongboxSecurities))

		sort.Sort(SecurityArrayStruct(PledgeeSegregatedSecurities))

		sort.Sort(SecurityArrayStruct(CombinedSecurities))

		//-----------------------------------------------------------------------------

		// Start Allocatin & Rearrangment
		// ReallocatedSecurities -> Structure where securites to reallocate will be stored
		// CombinedSecurities will only be used to read securities in order. Actual Changes will be 
		//	done in PledgerLongboxSecurities & PledgeeSegregatedSecurities

		// RQVEligibleValue[CollateralType] contains the max eligible vaule for each type
		RQVEligibleValueLeft := RQVEligibleValue
		RQVLeft := RQV

		SecuritiesChanged := make(map[string]float64)

		var ReallocatedSecurities []Securities

		// Iterating through all the securities 
		// Label: CombinedSecuritiesIterator --> to be used for break statements
		CombinedSecuritiesIterator: for _,valueSecurity := range CombinedSecurities {
			if RQVLeft > 0 {
				// More Security need to be taken out
				temp1 := RQVEligibleValueLeft[valueSecurity.CollateralForm]
				temp2,errBool:= strconv.ParseFloat(valueSecurity.MTM,64)
				if errBool != nil { fmt.Println(errBool) }
				if temp1 >= temp2 {
					// At least one more this type of collateralForm to be taken out
					temp3,errBool :=strconv.ParseFloat(valueSecurity.EffectiveValueinUSD,64)
					if errBool != nil { fmt.Println(errBool) }
					temp4 := RQVEligibleValueLeft[valueSecurity.CollateralForm]
					if temp3 <= temp4 {
						// All Security of this type will re allocated if RQV has balance

						temp4,errBool :=strconv.ParseFloat(valueSecurity.EffectiveValueinUSD,64)
						if errBool != nil { fmt.Println(errBool) }
						if temp4 <= RQVLeft {
							// All Security of this type will re allocated as RQV has balance
							
							RQVLeft -= temp4
							RQVEligibleValueLeft[valueSecurity.CollateralForm] -= temp4
							ReallocatedSecurities = append(ReallocatedSecurities, valueSecurity)
							
							temp5,errBool :=strconv.ParseFloat(valueSecurity.SecuritiesQuantity,64)
							if errBool != nil { fmt.Println(errBool) }
							SecuritiesChanged[valueSecurity.SecurityId] = temp5


						} else {
							// RQV has insufficient balance to take all securities
							temp5,errBool:= strconv.ParseFloat(valueSecurity.MTM,64)
							if errBool != nil { fmt.Println(errBool) }

							QuantityToTakeout := math.Ceil(RQVLeft / temp5)
							EffectiveValueinUSDtoAllocate := QuantityToTakeout * temp5

							RQVLeft -= EffectiveValueinUSDtoAllocate
							RQVEligibleValueLeft[valueSecurity.CollateralForm] -= EffectiveValueinUSDtoAllocate
							tempSecurity2 := valueSecurity 
							tempSecurity2.SecuritiesQuantity = strconv.FormatFloat(QuantityToTakeout, 'E', -1, 64)
							tempSecurity2.EffectiveValueinUSD = strconv.FormatFloat(EffectiveValueinUSDtoAllocate, 'E', -1, 64)
							ReallocatedSecurities = append(ReallocatedSecurities, valueSecurity)
							SecuritiesChanged[valueSecurity.SecurityId] = QuantityToTakeout
						}
					} else {
						// We can take out more of this type of CollateralForm but not all
						
						temp5,errBool:= strconv.ParseFloat(valueSecurity.MTM,64)
							if errBool != nil { fmt.Println(errBool) }
						QuantityToTakeout := math.Ceil(float64(RQVEligibleValueLeft[valueSecurity.CollateralForm] / float64(temp5)))
						EffectiveValueinUSDtoAllocate := QuantityToTakeout * float64(temp5)

						if EffectiveValueinUSDtoAllocate >= float64(RQVLeft) {
							// Can takeout the Securites 

							RQVLeft -= EffectiveValueinUSDtoAllocate
							RQVEligibleValueLeft[valueSecurity.CollateralForm] -= float64(EffectiveValueinUSDtoAllocate)
							tempSecurity2 := valueSecurity 
							tempSecurity2.SecuritiesQuantity= strconv.FormatFloat(QuantityToTakeout, 'E', -1, 64)
							//strconv.ParseFloat(QuantityToTakeout)
							tempSecurity2.EffectiveValueinUSD= strconv.FormatFloat(EffectiveValueinUSDtoAllocate, 'E', -1, 64)
							//strconv.ParseFloat(EffectiveValueinUSDtoAllocate)
							ReallocatedSecurities = append(ReallocatedSecurities, valueSecurity)
							SecuritiesChanged[valueSecurity.SecurityId] = float64(QuantityToTakeout)

						} else {
							// Cannot takeout all possble Securities as RQV balance is low
							temp6,errBool:= strconv.ParseFloat(valueSecurity.MTM,64)
							if errBool != nil { fmt.Println(errBool) }

							if QuantityToTakeout >math.Ceil(float64(RQVLeft / temp6)) {
								QuantityToTakeout = math.Ceil(float64(RQVLeft / temp6))
							}
							EffectiveValueinUSDtoAllocate = QuantityToTakeout * float64(temp6)

							RQVLeft -= EffectiveValueinUSDtoAllocate
							RQVEligibleValueLeft[valueSecurity.CollateralForm] -= float64(EffectiveValueinUSDtoAllocate)
							tempSecurity2 := valueSecurity 
							tempSecurity2.SecuritiesQuantity = strconv.FormatFloat(QuantityToTakeout, 'E', -1, 64)
							tempSecurity2.EffectiveValueinUSD = strconv.FormatFloat(EffectiveValueinUSDtoAllocate, 'E', -1, 64)
							ReallocatedSecurities = append(ReallocatedSecurities, valueSecurity)
							SecuritiesChanged[valueSecurity.SecurityId] = float64(QuantityToTakeout)						
						}
					}

				} else{
					// We Cannot take out more of this type of Security so SKIP
				}
			} else {
				// Security cutting done
				// Break from the CombinedSecuritiesIterator as RQV is less than 0
				break CombinedSecuritiesIterator
			}
		}
		
		//-----------------------------------------------------------------------------
		
		// Flushing securities from both Accounts
		// remove_securitiesFromAccount
		function = "remove_securitiesFromAccount"

		invokeArgs := util.ToChaincodeArgs(function, PledgerLongboxAccount)
		result, err := stub.InvokeChaincode(AccountChainCode, invokeArgs)
		if err != nil {
			errStr := fmt.Sprintf("Failed to flush " + PledgerLongboxAccount+" from 'Account' chaincode. Got error: %s", err.Error())
			fmt.Printf(errStr)
			return nil, errors.New(errStr)
		}
		fmt.Print("Transaction hash returned: ");
		fmt.Println(result)
		invokeArgs = util.ToChaincodeArgs(function, PledgeeSegregatedAccount)
		result, err = stub.InvokeChaincode(AccountChainCode, invokeArgs)
		if err != nil {
			errStr := fmt.Sprintf("Failed to flush " + PledgeeSegregatedAccount+" from 'Account' chaincode. Got error: %s", err.Error())
			fmt.Printf(errStr)
			return nil, errors.New(errStr)
		}
		
		//-----------------------------------------------------------------------------

		// Committing the state to Blockchain

		// Function from Account Chaincode for 
		functionUpdateSecurity 	:= "update_security" 		// Securities Object
		//functionDeleteSecurity	:= "delete_security"	// SecurityId, AccountNumber
		functionAddSecurity 	:= "add_security"			// Security Object

		// Update the existing Securities for Pledger Longbox A/c
		for _,valueSecurity := range CombinedSecurities {
				_SecurityQuantity, err := strconv.ParseFloat(valueSecurity.SecuritiesQuantity,64)
				if err != nil {
					errStr := fmt.Sprintf("Failed to convert SecurityQuantity(string) to SecurityQuantity(int). Got error: %s", err.Error())
					fmt.Printf(errStr)
				}
				_SecurityId := SecuritiesChanged[valueSecurity.SecurityId]
				newQuantity := _SecurityQuantity - _SecurityId
				if _SecurityQuantity != _SecurityId {
					
					order := 	`[`+
						`"` + valueSecurity.SecurityId + `", `+
						`"` + PledgerLongboxAccount + `", `+													
						`"` + valueSecurity.SecuritiesName + `", `+
						`"` + strconv.FormatFloat(newQuantity, 'E', -1, 64) + `", `+
						`"` + valueSecurity.SecurityType + `", `+
						`"` + valueSecurity.CollateralForm + `", `+
						`"` + valueSecurity.CollateralForm + `", `+
						`"", `+
						`"` + valueSecurity.MTM + `", `+
						`"` + valueSecurity.EffectivePercentage + `", `+
						`"` + valueSecurity.EffectiveValueinUSD + `", `+
						`"` + valueSecurity.Currency + `" `+
						`]`

					invokeArgs := util.ToChaincodeArgs(functionUpdateSecurity, order)
					result, err := stub.InvokeChaincode(AccountChainCode, invokeArgs)
					if err != nil {
						errStr := fmt.Sprintf("Failed to update Security from 'Account' chaincode. Got error: %s", err.Error())
						fmt.Printf(errStr)
						return nil, errors.New(errStr)
					}
					fmt.Print("Transaction hash returned: ");
					fmt.Println(result)
				}
			
		}

		// Update the new Securities to Pledgee Segregated A/c
		for _, valueSecurity := range ReallocatedSecurities {
			
			order := 	`[`+
				`"` + valueSecurity.SecurityId + `" ,`+
				`"` + PledgeeSegregatedAccount + `" ,`+													
				`"` + valueSecurity.SecuritiesName + `" ,`+
				`"` + valueSecurity.SecuritiesQuantity + `" ,`+
				`"` + valueSecurity.SecurityType + `" ,`+
				`"` + valueSecurity.CollateralForm + `" ,`+
				`"` + valueSecurity.CollateralForm + `" ,`+
				`"" ,`+
				`"` + valueSecurity.MTM + `" ,`+
				`"` + valueSecurity.EffectivePercentage + `" ,`+
				`"` + valueSecurity.EffectiveValueinUSD + `" ,`+
				`"` + valueSecurity.Currency + `" `+
				`]`


			invokeArgs := util.ToChaincodeArgs(functionAddSecurity, order)
			result, err := stub.InvokeChaincode(AccountChainCode, invokeArgs)
			if err != nil {
				errStr := fmt.Sprintf("Failed to update Security from 'Account' chaincode. Got error: %s", err.Error())
				fmt.Printf(errStr)
				return nil, errors.New(errStr)
			}
			fmt.Print("Transaction hash returned: ");
			fmt.Println(result)
		}


		//-----------------------------------------------------------------------------

		// Update Transaction data finally

		fmt.Println("Successfully updated allocation status to 'Allocation Successful'")
		ConversionRateAsBytes, _ := json.Marshal(ConversionRate)								//marshal an emtpy array of strings to clear the index
		
		order:= `[` +
			`"` + TransactionData.TransactionId + `" , ` + 
			`"` + TransactionData.TransactionDate + `" , ` + 
			`"` + TransactionData.DealID + `" , ` + 
			`"` + TransactionData.Pledger + `" , ` + 
			`"` + TransactionData.Pledgee + `" , ` + 
			`"` + TransactionData.RQV + `" , ` +
	        `"` + TransactionData.Currency + `" , ` + 
	        `"` + string(ConversionRateAsBytes) + `" , ` +  
	        `"` + MarginCallTimpestamp + `" , ` + 
	        "Allocation Successful" + ` , ` + 
	       	"Complete" + 
	        `]`
	    f := "update_transaction"
		invoke_args := util.ToChaincodeArgs(f, order)
		res, err := stub.InvokeChaincode(DealChaincode, invoke_args)
		if err != nil {
			errStr := fmt.Sprintf("Failed to invoke chaincode. Got error: %s", err.Error())
			fmt.Printf(errStr)
			return nil, errors.New(errStr)
		} 	
		fmt.Printf("Update transaction returned hash: %s",res)
	    
	    // Actual return of process end. 
		ret:= "{ \"message\" : \" + TransactionDataTransactionID + \" Completed allocation succcessfully.\", \"code\" : \"200\" }"
		return []byte(ret), nil

	}

	fmt.Println("end start_allocation")
	return nil, nil
}
