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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/util"
	"math"
	"net/http"
	//"net/url"
	"sort"
	"strconv"
)

type ManageAllocations struct {
}

type Transactions struct {
	TransactionId          string `json:"transactionId"`
	TransactionDate        string `json:"transactionDate"`
	DealID                 string `json:"dealId"`
	Pledger                string `json:"pledger"`
	Pledgee                string `json:"pledgee"`
	RQV                    string `json:"rqv"`
	Currency               string `json:"currency"`
	CurrencyConversionRate string `json:"currencyConversionRate"`
	MarginCAllDate         string `json:"marginCAllDate"`
	AllocationStatus       string `json:"allocationStatus"`
	TransactionStatus      string `json:"transactionStatus"`
	ComplianceStatus      string `json:"complianceStatus"`
}

type Deals struct { // Attributes of a Allocation
	DealID                       string `json:"dealId"`
	Pledger                      string `json:"pledger"`
	Pledgee                      string `json:"pledgee"`
	MaxValue                     string `json:"maxValue"` //Maximum Value of all the securities of each Collateral Form
	TotalValueLongBoxAccount     string `json:"totalValueLongBoxAccount"`
	TotalValueSegregatedAccount  string `json:"totalValueSegregatedAccount"`
	IssueDate                    string `json:"issueDate"`
	LastSuccessfulAllocationDate string `json:"lastSuccessfulAllocationDate"`
	Transactions                 string `json:"transactions"`
}

type Accounts struct {
	AccountID     string `json:"accountId"`
	AccountName   string `json:"accountName"`
	AccountNumber string `json:"accountNumber"`
	AccountType   string `json:"accountType"`
	TotalValue    string `json:"totalValue"`
	Currency      string `json:"currency"`
	Pledger       string `json:"pledger"`
	Securities    string `json:"securities"`
}

type Securities struct {
	SecurityId          string `json:"securityId"`
	AccountNumber       string `json:"accountNumber"`
	SecuritiesName      string `json:"securityName"`
	SecuritiesQuantity  string `json:"securityQuantity"`
	SecurityType        string `json:"securityType"`
	CollateralForm      string `json:"collateralForm"`
	TotalValue          string `json:"totalValue"`
	ValuePercentage     string `json:"valuePercentage"`
	MTM                 string `json:"mtm"`
	EffectivePercentage string `json:"effectivePercentage"`
	EffectiveValueChanged string `json:"effectiveValueChanged"`
	Currency            string `json:"currency"`
}

// Use as Object.Security["CommonStocks"][0]
// Reference [Tested by Pranav] https://play.golang.org/p/JlQJF5Z14X
type Ruleset struct {
	Security         map[string]map[string]float64 `json:"Security"`
	BaseCurrency     string               `json:"BaseCurrency"`
	EligibleCurrency []string             `json:"EligibleCurrency"`
}
// Varaible record to be filled with the data from the JSON
var rulesetFetched Ruleset

// Used for Security Array Sort
// Reference at https://play.golang.org/p/Rz9NCEVhGu
type SecurityArrayStruct []Securities 

func (slice SecurityArrayStruct) Len() int             { return len(slice) }
func (slice SecurityArrayStruct) Less(i, j int) bool { // Sorting through the field 'Priority'
	return rulesetFetched.Security[slice[i].CollateralForm]["Priority"] < rulesetFetched.Security[slice[j].CollateralForm]["Priority"]
}
func (slice SecurityArrayStruct) Swap(i, j int) { slice[i], slice[j] = slice[j], slice[i] }


// Use as Object.Rates["EUR"]
// Reference [Tested by Pranav] https://play.golang.org/p/j5Act-jN5C
type CurrencyConversion struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

// To be used as SecurityJSON["CommonStocks"]["Priority"] ==> 1
var SecurityJSON = map[string]map[string]string{
	"Common Stocks":         map[string]string{"Concentration Limit": "40", "Priority": "1", "Valuation Percentage": "97"},
	"Corporate Bonds":       map[string]string{"Concentration Limit": "30", "Priority": "2", "Valuation Percentage": "97"},
	"Sovereign Bonds":       map[string]string{"Concentration Limit": "25", "Priority": "3", "Valuation Percentage": "95"},
	"US Treasury Bills":      map[string]string{"Concentration Limit": "25", "Priority": "4", "Valuation Percentage": "95"},
	"US Treasury Bonds":      map[string]string{"Concentration Limit": "25", "Priority": "5", "Valuation Percentage": "95"},
	"US Treasury Notes":      map[string]string{"Concentration Limit": "25", "Priority": "6", "Valuation Percentage": "95"},
	"Gilt":                 map[string]string{"Concentration Limit": "25", "Priority": "7", "Valuation Percentage": "94"},
	"Federal Agency Bonds":   map[string]string{"Concentration Limit": "20", "Priority": "8", "Valuation Percentage": "93"},
	"Global Bonds":          map[string]string{"Concentration Limit": "20", "Priority": "9", "Valuation Percentage": "92"},
	"Preferred Shares":     map[string]string{"Concentration Limit": "20", "Priority": "10", "Valuation Percentage": "91"},
	"Convertible Bonds":     map[string]string{"Concentration Limit": "20", "Priority": "11", "Valuation Percentage": "90"},
	"Revenue Bonds":         map[string]string{"Concentration Limit": "15", "Priority": "12", "Valuation Percentage": "90"},
	"Medium Term Note":       map[string]string{"Concentration Limit": "15", "Priority": "13", "Valuation Percentage": "89"},
	"Short Term Investments": map[string]string{"Concentration Limit": "15", "Priority": "14", "Valuation Percentage": "87"},
	"Builder Bonds":         map[string]string{"Concentration Limit": "15", "Priority": "15", "Valuation Percentage": "85"}}

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
	err = stub.PutState("abc", []byte(msg)) //making a test var "abc", I find it handy to read/write to it right away to test the network
	if err != nil {
		return nil, err
	}
	var empty []string
	jsonAsBytes, _ := json.Marshal(empty) //marshal an emtpy array of strings to clear the index
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
	if function == "init" { // Initialize the chaincode state, used as reset
		return t.Init(stub, "init", args)
	} else if function == "start_allocation" { // Create a new Allocation
		return t.start_allocation(stub, args)
	} else if function == "LongboxAccountUpdated" { // Secondary Fire when Longbox account is updated
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

	_DealChaincode := args[0]
	_AccountName := args[1]
	_Role := args[2]
	_CurrentTimeStamp := args[3]

	fmt.Println("args: ", args)
	var TransactionsDataFetched []Transactions

	// Fetching Attl transactions for the user
	function := "getTransactions_byUser"
	QueryArgs := util.ToChaincodeArgs(function, _AccountName, _Role)
	result, err := stub.QueryChaincode(_DealChaincode, QueryArgs)
	if err != nil {
		errStr := fmt.Sprintf("Error in fetching Transactions from 'Deal' chaincode. Got error: %s", err.Error())
		fmt.Printf(errStr)
		return nil, errors.New(errStr)
	}
	json.Unmarshal(result, &TransactionsDataFetched)

	// Timestamp to Date/Time Objest in Go and Logic behind cutoff time
	// Ref: https://play.golang.org/p/KJRigmHzu9

	_CurrentTimeStampHour, err := strconv.ParseInt(_CurrentTimeStamp, 10, 64)
	if err != nil {
		panic(err)
	}
	var newTxStatus, newAllStatus string

	for _, ValueTransaction := range TransactionsDataFetched {

		if ValueTransaction.AllocationStatus == "Pending due to insufficient collateral" {

			if _CurrentTimeStampHour <=15 && _CurrentTimeStampHour >= 0 {
				// New securites are uploaded in cutoff time
				newTxStatus = "Ready"
				newAllStatus = "Ready for Allocation"
			} else {
				// New securities not uploaded in cutoff time
				newTxStatus = "Failed"
				newAllStatus = "Allocation Failed"
			}

			// Update allocation status of a transaction
			function = "update_transaction"
			invokeArgs := util.ToChaincodeArgs(function,
				ValueTransaction.TransactionId,
				ValueTransaction.TransactionDate,
				ValueTransaction.DealID,
				ValueTransaction.Pledger,
				ValueTransaction.Pledgee,
				ValueTransaction.RQV,
				ValueTransaction.Currency,
				"\""+ValueTransaction.CurrencyConversionRate+"\"",
				ValueTransaction.MarginCAllDate,
				newAllStatus,
				newTxStatus,
				"NA")
			fmt.Println(ValueTransaction)
			result, err := stub.InvokeChaincode(_DealChaincode, invokeArgs)
			if err != nil {
				errStr := fmt.Sprintf("Failed to update Transaction status from 'Deal' chaincode. Got error: %s", err.Error())
				fmt.Printf(errStr)
				return nil, errors.New(errStr)
			}
			fmt.Println("Transaction hash returned: ", result)
			fmt.Println(ValueTransaction.TransactionId + " updated with AllocationStatus as " + newAllStatus)
			fmt.Println(ValueTransaction.TransactionId + " updated with TransactionStatus as " + newTxStatus)

			//Sending event call
			tosend := "{ \"transactionId\" : \"" + ValueTransaction.TransactionId + "\", \"message\" : \"Transaction updated succcessfully with Allocation Status as " + newAllStatus + " \", \"code\" : \"200\"}"
			err = stub.SetEvent("evtsender", []byte(tosend))
			if err != nil {
				return nil, err
			}
		} else if ValueTransaction.TransactionStatus == "Ready for Allocation" {
			//Sending event call
			tosend := "{ \"transactionId\" : \"" + ValueTransaction.TransactionId + "\", \"message\" : \"Transaction updated succcessfully with Allocation Status as 'Ready for Allocation' \", \"code\" : \"200\"}"
			err = stub.SetEvent("evtsender", []byte(tosend))
			if err != nil {
				return nil, err
			}
		}
	}

	fmt.Println("end LongboxAccountUpdated")
	return nil, nil
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
	DealChaincode := args[0]
	AccountChainCode := args[1]
	APIIP := args[2]
	DealID := args[3]
	TransactionID := args[4]
	PledgerLongboxAccount := args[5]
	PledgeeSegregatedAccount := args[6]
	MarginCallTimpestamp := args[7]

	// Json to create report
	reportInJson := `{`

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
	fmt.Println(DealData)
	if DealData.DealID == DealID {
		fmt.Println("Deal found with DealID : " + DealID)
	} else {
		errMsg := "{ \"message\" : \"" + DealID + " Not Found.\", \"code\" : \"503\"}"
		err = stub.SetEvent("errEvent", []byte(errMsg))
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	Pledger := DealData.Pledger
	Pledgee := DealData.Pledgee
	fmt.Println("Pledger : ", Pledger)
	fmt.Println("Pledgee : ", Pledgee)

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
	fmt.Println(TransactionData)
	if TransactionData.TransactionId == TransactionID {
		fmt.Println("Transaction found with TransactionID : " + TransactionID)
	} else {
		errMsg := "{ \"message\" : \"" + TransactionID + " Not Found.\", \"code\" : \"503\"}"
		err = stub.SetEvent("errEvent", []byte(errMsg))
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
	/*RQV,errBool := strconv.ParseFloat(TransactionData.RQV)*/
	RQV, errBool := strconv.ParseFloat(TransactionData.RQV, 64)
	if errBool != nil {
		fmt.Println(errBool)
	}

	fmt.Println("RQV : ", RQV)
	// RQV currency of a deal
	RQVCurrency := TransactionData.Currency
	fmt.Println("RQVCurrency : ", RQVCurrency)
	//-----------------------------------------------------------------------------

	reportInJson += `"Deal ID" : "` + DealID + `",`
	reportInJson += `"Transaction ID" : "` + TransactionID + `",`
	reportInJson += `"Margin Call Date" : "` + MarginCallTimpestamp + `",`
	reportInJson += `"Pledgee" : "` + Pledgee + `",`
	reportInJson += `"Pledger" : "` + Pledger + `",`
	reportInJson += `"Pledger Longbox Account" : "` + PledgerLongboxAccount + `",`
	reportInJson += `"Pledgee Segregated Account" : "` + PledgeeSegregatedAccount + `",`
	reportInJson += `"RQV" : "` + strconv.FormatFloat(RQV, 'f', 2, 64) + `",`
	reportInJson += `"Currency" : "` + TransactionData.Currency + `",`

	// SecurityJSON to String https://play.golang.org/p/_C21BONfZk
	reportInJson += `"Public Rule Set" : {"US Treasury Bills":{"Valuation Percentage":"95","Concentration Limit":"25","Priority":"4"},"US Treasury Notes":{"Concentration Limit":"25","Priority":"6","Valuation Percentage":"95"},"Gilt":{"Priority":"7","Valuation Percentage":"94","Concentration Limit":"25"},"Common Stocks":{"Valuation Percentage":"97","Concentration Limit":"40","Priority":"1"},"Federal Agency Bonds":{"Concentration Limit":"20","Priority":"8","Valuation Percentage":"93"},"Convertible Bonds":{"Concentration Limit":"20","Priority":"11","Valuation Percentage":"90"},"Revenue Bonds":{"Concentration Limit":"15","Priority":"12","Valuation Percentage":"90"},"Medium Term Note":{"Priority":"13","Valuation Percentage":"89","Concentration Limit":"15"},"Corporate Bonds":{"Valuation Percentage":"97","Concentration Limit":"30","Priority":"2"},"Global Bonds":{"Concentration Limit":"20","Priority":"9","Valuation Percentage":"92"},"Builder Bonds":{"Concentration Limit":"15","Priority":"15","Valuation Percentage":"85"},"Sovereign Bonds":{"Concentration Limit":"25","Priority":"3","Valuation Percentage":"95"},"US Treasury Bonds":{"Priority":"5","Valuation Percentage":"95","Concentration Limit":"25"},"Preferrred Shares":{"Concentration Limit":"20","Priority":"10","Valuation Percentage":"91"},"Short Term Investments":{"Valuation Percentage":"87","Concentration Limit":"15","Priority":"14"}} ,`
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
	fmt.Print("Transaction hash returned: ")
	fmt.Println(result)
	fmt.Println("Successfully updated allocation status to 'Allocation in progress'")

	//-----------------------------------------------------------------------------

	// Fetching the Private Securtiy Ruleset based on Pledger & Pledgee
	// Escaping the values to be put in URL
	//PledgerESC := url.QueryEscape(Pledger)
	//PledgeeESC := url.QueryEscape(Pledgee)

	url := fmt.Sprintf("http://%s/securityRuleset/%s/%s", APIIP, Pledger, Pledgee)
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
		errMsg := "{ \"message\" : \"Unable to fetch Security Ruleset at " + APIIP + ".\", \"code\" : \"503\"}"
		err = stub.SetEvent("errEvent", []byte(errMsg))
		if err != nil {
			return nil, err
		}
	}

	fmt.Println("The SecurityRuleset response is::" + strconv.Itoa(resp.StatusCode))

	

	// Use json.Decode for reading streams of JSON data and store it
	if err := json.NewDecoder(resp.Body).Decode(&rulesetFetched); err != nil {
		fmt.Println(err)
	}

	resbody, err := json.Marshal(rulesetFetched)
	if err != nil {
		fmt.Println(err)
	}
	reportInJson += `"Private Rule set" : ` + string(resbody) + `,`

	// Callers should close resp.Body when done reading from it
	// Defer the closing of the body
	defer resp.Body.Close()

	fmt.Println("Ruleset : ")
	fmt.Println(rulesetFetched)
	fmt.Println(resbody)

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
	url2 := fmt.Sprintf("http://api.fixer.io/latest?base=" + RQVCurrency)

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
		errMsg := "{ \"message\" : \"Unable to fetch Currency Exchange Rates from: " + url2 + ".\", \"code\" : \"503\"}"
		err2 = stub.SetEvent("errEvent", []byte(errMsg))
		if err2 != nil {
			return nil, err2
		}
	}

	fmt.Println("The SecurityRuleset response is::" + strconv.Itoa(resp2.StatusCode))

	// Varaible ConversionRate to be filled with the data from the JSON
	var ConversionRate CurrencyConversion

	// Use json.Decode for reading streams of JSON data and store it
	if err := json.NewDecoder(resp2.Body).Decode(&ConversionRate); err != nil {
		fmt.Println(err)
	}

	respbody, err := json.Marshal(ConversionRate)
	if err != nil {
		fmt.Println(err)
	}
	reportInJson += `"Currency Conversion Rate" : ` + string(respbody) + `,`

	// Callers should close resp.Body when done reading from it
	// Defer the closing of the body
	defer resp2.Body.Close()

	fmt.Println("Exchange Rate : ")
	fmt.Println(ConversionRate)

	//-----------------------------------------------------------------------------

	// Caluculate eligible Collateral value from RQV
	RQVEligibleValue := make(map[string]float64)

	//Iterating through all the securities present in the ruleset
	for key, value := range rulesetFetched.Security {
		// key = "CommonStocks" && value = [35, 1, 95]
		// value[0] => ConcentrationLimit
		// value[1] => Priority
		// value[2] => ValuationPercentage
		ConcentrationLimitPri := value["Concentration Limit"]
		RQVEligibleValue[key] = (RQV * ConcentrationLimitPri) / 100
		//fmt.Printf("inside checking privateRuleset is subset of publicRuleSet..")
		//fmt.Printf("%#v", RQVEligibleValue[key])		
	}
	fmt.Println("RQVEligibleValue after calculation:")
	fmt.Printf("%#v", RQVEligibleValue)
	fmt.Println()

	//-----------------------------------------------------------------------------

	// Fetch Pledger & Pledgee securities for longbox and segregated accounts
	function = "getSecurities_byAccount"

	queryArgs = util.ToChaincodeArgs(function, PledgerLongboxAccount)
	PledgerLongboxSecuritiesString, err := stub.QueryChaincode(AccountChainCode, queryArgs)

	queryArgs = util.ToChaincodeArgs(function, PledgeeSegregatedAccount)
	PledgeeSegregatedSecuritiesString, err := stub.QueryChaincode(AccountChainCode, queryArgs)

	/**	Calculate the effective value and total value of each Security present in the Longbox account of the pledger
	and the Segregated account of the pledgee
	*/
	var TotalValuePledgerLongbox, TotalValuePledgeeSegregated, AvailableEligibleCollateral float64
	var PledgerLongboxSecurities, PledgeeSegregatedSecurities, CombinedSecurities []Securities

	// Make inteface to receive string. UnMarshal them extract them and make an array out of them.
	var PledgerLongboxSecuritiesJSON, PledgeeSegregatedSecuritiesJSON SecurityArrayStruct
	json.Unmarshal(PledgerLongboxSecuritiesString, &PledgerLongboxSecuritiesJSON)
	json.Unmarshal(PledgeeSegregatedSecuritiesString, &PledgeeSegregatedSecuritiesJSON)

	TotalValuePledgerLongboxSecurities := make(map[string]float64)
	TotalValuePledgeeSegregatedSecurities := make(map[string]float64)
	AvailableCollateral := make(map[string]float64)
	AvailableEligible := make(map[string]float64)

	fmt.Println("PledgerLongboxSecuritiesJSON after calculation:")
	fmt.Printf("%#v", PledgerLongboxSecuritiesJSON)
	fmt.Println()
	fmt.Println("PledgeeSegregatedSecuritiesJSON after calculation:")
	fmt.Printf("%#v", PledgeeSegregatedSecuritiesJSON)
	fmt.Println()
	//Operations for Pledger Longbox Securities
	for _, value := range PledgerLongboxSecuritiesJSON {
		// Key = Security ID && value = Security Structure
		tempSecurity := Securities{}
		tempSecurity = value

		// Check if Current Collateral Form type is acceptied in ruleset. If not skip it!
		if len(rulesetFetched.Security[tempSecurity.CollateralForm]) > 0 {

			url2 := fmt.Sprintf("http://" + APIIP + "/MarketData/" + tempSecurity.SecurityId)

			// Build the request
			req2, err2 := http.NewRequest("GET", url2, nil)
			if err2 != nil {
				fmt.Println("Market rate fetch error: ", err2)
				return nil, err2
			}

			// For control over HTTP client headers, redirect policy, and other settings, create a Client
			// A Client is an HTTP cliPledgeeSegregatedSecuritiesent
			client2 := &http.Client{}

			// Send the request via a client
			// Do sends an HTTP request and returns an HTTP response
			resp2, err2 := client2.Do(req2)
			if err2 != nil {
				fmt.Println("Do: ", err2)
				errMsg := "{ \"message\" : \"Unable to fetch Market Rates from: " + url2 + ".\", \"code\" : \"503\"}"
				err2 = stub.SetEvent("errEvent", []byte(errMsg))
				if err2 != nil {
					return nil, err2
				}
			}

			fmt.Println("The MarketData response is::" + strconv.Itoa(resp2.StatusCode))

			var stringArr []string

			// Use json.Decode for reading streams of JSON data and store it
			if err := json.NewDecoder(resp2.Body).Decode(&stringArr); err != nil {
				fmt.Println(err)
			}
			// Callers should close resp.Body when done reading from it
			// Defer the closing of the body
			defer resp2.Body.Close()

			tempSecurity.MTM = stringArr[0]
			// Storing the Value percentage in the security ruleset data itself
			tempSecurity.ValuePercentage = strconv.FormatFloat(rulesetFetched.Security[tempSecurity.CollateralForm]["Valuation Percentage"], 'f', 2, 64)
			//convert valuePercentage(string) to float
			tempValuePercentage, errBool := strconv.ParseFloat(tempSecurity.ValuePercentage, 64)
			if errBool != nil {
				fmt.Println(errBool)
			}

			temp, errBool := strconv.ParseFloat(tempSecurity.MTM, 64)
			if errBool != nil {
				fmt.Println(errBool)
			}

			_rate := ConversionRate.Rates[tempSecurity.Currency]
			if tempSecurity.Currency == RQVCurrency {
				_rate = 1
			}

			fmt.Println("_rate")
			fmt.Println(_rate)
			// change mtm to appropriate float format
			//calculate exchange rate for mtm
			_changedMTM :=  temp/_rate
			fmt.Println("_changedMTM")
			fmt.Println(_changedMTM)
			// Effective Value =  (MTM(market Value) * valuePercentage)/100
			temp3 := (_changedMTM * tempValuePercentage)/100
			fmt.Println("temp3")
			fmt.Println(temp3)
			tempSecurity.EffectiveValueChanged = strconv.FormatFloat(temp3, 'f', 2, 64)
			// Adding it to TotalValue
			temp2, errBool := strconv.ParseFloat(tempSecurity.SecuritiesQuantity, 64)
			if errBool != nil {
				fmt.Println(errBool)
			}
			// Calculate Total Value = Effective Value * Quantity
			tempTotal := temp3 * temp2

			tempSecurity.TotalValue = strconv.FormatFloat(tempTotal, 'f', 2, 64)
			fmt.Println("tempSecurity.TotalValue")
			fmt.Println(tempSecurity.TotalValue)
			// Calculate Total value based on Collateral form
			TotalValuePledgerLongboxSecurities[tempSecurity.CollateralForm] += tempTotal
			// Calculate Total value of pledger's longbox account
			TotalValuePledgerLongbox += tempTotal
			// Calculate the total value of all the securities based on Collateral form
			//AvailableCollateral[tempSecurity.CollateralForm] += tempTotal

			// Calculate Available Eligiblex = Minimum (Available[tempSecurity.CollateralForm], Eligible[tempSecurity.CollateralForm])
			//AvailableEligible[tempSecurity.CollateralForm] = math.Min(AvailableCollateral[tempSecurity.CollateralForm],RQVEligibleValue[tempSecurity.CollateralForm])
			
			/*	Warning :
				Saving Priority for the Security in filed `ValuePercentage`
				This is just for using the limited sorting application provided by GOlang
				By no chance is this to be stored on Blockchain.
			*/
			tempSecurity.ValuePercentage = strconv.FormatFloat(rulesetFetched.Security[tempSecurity.CollateralForm]["Valuation Percentage"], 'f', 2, 64)
			fmt.Println("tempSecurity.ValuePercentage")
			fmt.Println(tempSecurity.ValuePercentage)
			// Append Securities to an array
			PledgerLongboxSecurities = append(PledgerLongboxSecurities, tempSecurity)
			CombinedSecurities = append(CombinedSecurities, tempSecurity)
		}
	}
	

	// Operations for Pledgee Segregated Account(s)
	for _, value := range PledgeeSegregatedSecuritiesJSON {
		// Key = Security ID && value = Security Structure
		tempSecurity := Securities{}
		tempSecurity = value

		// Check if Current Collateral Form type is acceptied in ruleset. If not skip it!
		if len(rulesetFetched.Security[tempSecurity.CollateralForm]) > 0 {

			// Storing the Value percentage in the security data itself
			tempSecurity.ValuePercentage = SecurityJSON[tempSecurity.CollateralForm]["Valuation Percentage"]
			
			//convert valuePercentage(string) to float
			tempValuePercentage, errBool := strconv.ParseFloat(tempSecurity.ValuePercentage, 64)
			if errBool != nil {
				fmt.Println(errBool)
			}

			temp, errBool := strconv.ParseFloat(tempSecurity.MTM, 64)
			if errBool != nil {
				fmt.Println(errBool)
			}

			_rate := ConversionRate.Rates[tempSecurity.Currency]
			if tempSecurity.Currency == RQVCurrency {
				_rate = 1
			}

			fmt.Println("_rate")
			fmt.Println(_rate)
			//calculate Currency conversion rate(to RQVCurrency) for mtm  
			_changedMTM :=  temp/_rate
			fmt.Println("_changedMTM")
			fmt.Println(_changedMTM)
			// Effective Value =  (MTM(market Value) * valuePercentage)/100
			temp3 := (_changedMTM * tempValuePercentage)/100
			fmt.Println("temp3")
			fmt.Println(temp3)
			tempSecurity.EffectiveValueChanged = strconv.FormatFloat(temp3, 'f', 2, 64)
			// Adding it to TotalValue

			temp2, errBool := strconv.ParseFloat(tempSecurity.SecuritiesQuantity, 64)
			if errBool != nil {
				fmt.Println(errBool)
			}
			// Calculate Total Value = Effective Value * Quantity
			tempTotal := temp3 * temp2

			tempSecurity.TotalValue = strconv.FormatFloat(tempTotal, 'f', 2, 64)
			fmt.Println("tempSecurity.TotalValue")
			fmt.Println(tempSecurity.TotalValue)
			// Calculate Total value based on Collateral form
			TotalValuePledgeeSegregatedSecurities[tempSecurity.CollateralForm] += tempTotal
			
			// Calculate Total value of pledgee's segregated account
			TotalValuePledgeeSegregated += tempTotal
			// Calculate the total value of all the securities based on Collateral form
			//AvailableCollateral[tempSecurity.CollateralForm] += tempTotal

			// Calculate Available Eligiblex = Minimum (Available[tempSecurity.CollateralForm], Eligible[tempSecurity.CollateralForm])
			//AvailableEligible[tempSecurity.CollateralForm] = math.Min(AvailableCollateral[tempSecurity.CollateralForm],RQVEligibleValue[tempSecurity.CollateralForm])
			
			// Calculate Available Eligible Collateral = Sum (Available Eligible)
			//AvailableEligibleCollateral = AvailableEligibleCollateral + AvailableEligible[tempSecurity.CollateralForm]

			
			/*	Warning :
				Saving Priority for the Security in filed `ValuePercentage`
				This is just for using the limited sorting application provided by GOlang
				By no chance is this to be stored on Blockchain.
			*/
			tempSecurity.ValuePercentage = strconv.FormatFloat(rulesetFetched.Security[tempSecurity.CollateralForm]["Valuation Percentage"], 'f', 2, 64)
			fmt.Println("tempSecurity.ValuePercentage")
			fmt.Println(tempSecurity.ValuePercentage)
			// Append Securities to an array
			PledgeeSegregatedSecurities = append(PledgeeSegregatedSecurities, tempSecurity)
			CombinedSecurities = append(CombinedSecurities, tempSecurity)
		}

	}

	fmt.Println("TotalValuePledgeeSegregatedSecurities")
	fmt.Println(TotalValuePledgeeSegregatedSecurities)
	fmt.Println("TotalValuePledgeeSegregated")
	fmt.Println(TotalValuePledgeeSegregated)
	
	fmt.Println()
	fmt.Println("PledgerLongboxSecurities after calculation:")
	fmt.Printf("%#v", PledgerLongboxSecurities)
	fmt.Println()
	fmt.Println("PledgeeSegregatedSecurities after calculation:")
	fmt.Printf("%#v", PledgeeSegregatedSecurities)
	fmt.Println()
	fmt.Println("CombinedSecurities after calculation:")
	fmt.Printf("%#v", CombinedSecurities)
	fmt.Println()

	for _, valueSecurity := range CombinedSecurities {
			tempTotal, errBool := strconv.ParseFloat(valueSecurity.TotalValue, 64)
			if errBool != nil {
				fmt.Println(errBool)
			}
			// Calculate the total value of all the securities based on Collateral form
			AvailableCollateral[valueSecurity.CollateralForm] += tempTotal
	}

	for key := range AvailableCollateral {
		// Calculate Available Eligiblex = Minimum (Available[tempSecurity.CollateralForm], Eligible[tempSecurity.CollateralForm])
		AvailableEligible[key] = math.Min(AvailableCollateral[key],RQVEligibleValue[key])

		// Calculate Available Eligible Collateral = Sum (Available Eligible)
		AvailableEligibleCollateral = AvailableEligibleCollateral + AvailableEligible[key]
	}
	fmt.Println("AvailableCollateral after calculation:")
	fmt.Printf("%#v", AvailableCollateral)
	fmt.Println()
	fmt.Println("AvailableEligible")
	fmt.Println(AvailableEligible)
	fmt.Println()
	fmt.Println("AvailableEligibleCollateral")
	fmt.Println(AvailableEligibleCollateral)
	fmt.Println()
	//-----------------------------------------------------------------------------

	if AvailableEligibleCollateral < RQV {
		
		// Update transaction's allocation status to "Pending due to insufficient collateral" and transaction status to "Pending"
		f := "update_transaction"
		invoke_args := util.ToChaincodeArgs(f, TransactionData.TransactionId,TransactionData.TransactionDate, TransactionData.DealID, TransactionData.Pledger,TransactionData.Pledgee, TransactionData.RQV, TransactionData.Currency,"\" \"", TransactionData.MarginCAllDate, "Pending due to insufficient collateral","Matched","NA")
		fmt.Println(TransactionData);
		result, err := stub.InvokeChaincode(DealChaincode, invoke_args)
		if err != nil {
			errStr := fmt.Sprintf("Failed to invoke chaincode. Got error: %s", err.Error())
			fmt.Printf(errStr)
			return nil, errors.New(errStr)
		} 	
		fmt.Print("Update transaction returned : ")
		fmt.Println(result)
		fmt.Println("Successfully updated allocation status to 'Pending' due to insufficient collateral'")
	    //Send a event to event handler
	    tosend:= "{ \"transactionId\" : \"" + TransactionData.TransactionId + "\", \"message\" : \"Transaction Allocation updated succcessfully with status 'Pending' due to insufficient collateral.\", \"code\" : \"200\"}"
	    err = stub.SetEvent("evtsender", [] byte(tosend))
	    if err != nil {
	        return nil, err
	    }

	    // Actual return of process end. 
		
		return nil, nil

	} else {

		//-----------------------------------------------------------------------------
		
		// Sorting the Securities in PledgerLongboxSecurities & PledgeeSegregatedSecurities
		// Using Code defination like https://play.golang.org/p/ciN45THQjM
		// Reference from http://nerdyworm.com/blog/2013/05/15/sorting-a-slice-of-structs-in-go/

		sort.Sort(SecurityArrayStruct(PledgerLongboxSecurities))
		sort.Sort(SecurityArrayStruct(PledgeeSegregatedSecurities))
		sort.Sort(SecurityArrayStruct(CombinedSecurities))
		fmt.Println("CombinedSecurities after sort: ", CombinedSecurities)
		// Start Allocatin & Rearrangment
		// ReallocatedSecurities -> Structure where securites to reallocate will be stored
		// CombinedSecurities will only be used to read securities in order. Actual Changes will be 
		//	done in PledgerLongboxSecurities & PledgeeSegregatedSecurities

		// RQVEligibleValue[CollateralType] contains the max eligible vaule for each type
		RQVEligibleValueLeft := RQVEligibleValue
		RQVLeft := RQV

		SecuritiesAllocated := make(map[string]float64)
		TotalValueAllocated := make(map[string]float64)
		var ReallocatedSecurities []Securities
		
		// Iterating through all the securities 
		// Label: PledgerLongboxSecuritiesIterator --> to be used for break statements
		
		CombinedSecuritiesIterator:
		for _, valueSecurity := range CombinedSecurities {
			fmt.Println("RQVLeft: ", RQVLeft)
			fmt.Println("TotalValuePledgeeSegregated: ", TotalValuePledgeeSegregated)
			fmt.Println("TotalValuePledgerLongbox: ", TotalValuePledgerLongbox)
			//var TotalValuePledgee float64
			if RQVLeft > 0 {
				// More Security need to be taken out
				rqvEligibleValueLeft := RQVEligibleValueLeft[valueSecurity.CollateralForm]
				fmt.Println("rqvEligibleValueLeft: ",rqvEligibleValueLeft)
				totalValue, errBool := strconv.ParseFloat(valueSecurity.TotalValue, 64)
				if errBool != nil {
					fmt.Println(errBool)
				}
				fmt.Println("totalValue: ",totalValue)
				if rqvEligibleValueLeft > 0 {
					if totalValue <= rqvEligibleValueLeft {
						// At least one more this type of collateralForm to be taken out
						if totalValue <= RQVLeft {
							// All Security of this type will re allocated as RQV has balance

							RQVLeft -= totalValue
							fmt.Println("RQVLeft: ",RQVLeft)
							RQVEligibleValueLeft[valueSecurity.CollateralForm] -= totalValue
							fmt.Println(valueSecurity.CollateralForm +": ",RQVEligibleValueLeft[valueSecurity.CollateralForm])
							ReallocatedSecurities = append(ReallocatedSecurities, valueSecurity)
							fmt.Println("ReallocatedSecurities: ",ReallocatedSecurities)
							securityQuantity, errBool := strconv.ParseFloat(valueSecurity.SecuritiesQuantity, 64)
							if errBool != nil {
								fmt.Println(errBool)
							}
							fmt.Println("securityQuantity: ",securityQuantity)
							SecuritiesAllocated[valueSecurity.SecurityId] = securityQuantity
							fmt.Println(valueSecurity.SecurityId + ": " , SecuritiesAllocated[valueSecurity.SecurityId])
							TotalValueAllocated[valueSecurity.SecurityId] = totalValue
							fmt.Println(valueSecurity.SecurityId + ": " , TotalValueAllocated[valueSecurity.SecurityId])
							/*TotalValuePledgee += totalValue
							fmt.Println(TotalValuePledgee)*/
						}else {
							// RQV has insufficient balance to take all securities
							securityQuantity, errBool := strconv.ParseFloat(valueSecurity.SecuritiesQuantity, 64)
							if errBool != nil {
								fmt.Println(errBool)
							}
							fmt.Println("securityQuantity: ",securityQuantity)
							effectiveValueChanged, errBool := strconv.ParseFloat(valueSecurity.EffectiveValueChanged, 64)
							if errBool != nil {
								fmt.Println(errBool)
							}
							fmt.Println("effectiveValueChanged: ",effectiveValueChanged)
							QuantityToTakeout := math.Ceil((RQVLeft * securityQuantity)/ totalValue)
							fmt.Println("QuantityToTakeout: ", QuantityToTakeout)
							totalValueToAllocate := QuantityToTakeout * effectiveValueChanged
							fmt.Println(totalValueToAllocate)
							if totalValueToAllocate > rqvEligibleValueLeft {
								totalValueToAllocate = rqvEligibleValueLeft
							}
							RQVLeft -= totalValueToAllocate
							fmt.Println("RQVLeft: ",RQVLeft)
							RQVEligibleValueLeft[valueSecurity.CollateralForm] -= totalValueToAllocate
							fmt.Println("RQVEligibleValueLeft: ",RQVEligibleValueLeft)
							tempSecurity2 := valueSecurity
							tempSecurity2.SecuritiesQuantity = strconv.FormatFloat(QuantityToTakeout, 'f', 2, 64)
							tempSecurity2.TotalValue = strconv.FormatFloat(totalValueToAllocate, 'f', 2, 64)
							ReallocatedSecurities = append(ReallocatedSecurities, tempSecurity2)
							fmt.Println("ReallocatedSecurities: ",ReallocatedSecurities)
							SecuritiesAllocated[valueSecurity.SecurityId] = QuantityToTakeout
							fmt.Println(valueSecurity.SecurityId + ": " , SecuritiesAllocated[valueSecurity.SecurityId])
							TotalValueAllocated[valueSecurity.SecurityId] = totalValueToAllocate
							fmt.Println(valueSecurity.SecurityId + ": " , TotalValueAllocated[valueSecurity.SecurityId])
							/*TotalValuePledgee += totalValueToAllocate
							fmt.Println(TotalValuePledgee)*/
						}
					}else{
						// rqvEligibleValueLeft is less than total Value
						securityQuantity, errBool := strconv.ParseFloat(valueSecurity.SecuritiesQuantity, 64)
						if errBool != nil {
							fmt.Println(errBool)
						}
						fmt.Println("securityQuantity: ",securityQuantity)
						effectiveValueChanged, errBool := strconv.ParseFloat(valueSecurity.EffectiveValueChanged, 64)
						if errBool != nil {
							fmt.Println(errBool)
						}
						fmt.Println("effectiveValueChanged: ",effectiveValueChanged)
						QuantityToTakeout := math.Ceil((rqvEligibleValueLeft * securityQuantity)/ totalValue)
						fmt.Println("QuantityToTakeout: ", QuantityToTakeout)
						totalValueToAllocate := QuantityToTakeout * effectiveValueChanged
						fmt.Println("totalValueToAllocate: ", totalValueToAllocate)
						if totalValueToAllocate > rqvEligibleValueLeft {
							totalValueToAllocate = rqvEligibleValueLeft
						}
						RQVLeft -= totalValueToAllocate
						fmt.Println("RQVLeft: ",RQVLeft)
						RQVEligibleValueLeft[valueSecurity.CollateralForm] -= totalValueToAllocate
						fmt.Println("RQVEligibleValueLeft: ",RQVEligibleValueLeft)
						tempSecurity2 := valueSecurity
						tempSecurity2.SecuritiesQuantity = strconv.FormatFloat(QuantityToTakeout, 'f', 2, 64)
						tempSecurity2.TotalValue = strconv.FormatFloat(totalValueToAllocate, 'f', 2, 64)
						ReallocatedSecurities = append(ReallocatedSecurities, tempSecurity2)
						fmt.Println("ReallocatedSecurities: ",ReallocatedSecurities)

						SecuritiesAllocated[valueSecurity.SecurityId] = QuantityToTakeout
						fmt.Println(valueSecurity.SecurityId + ": " , SecuritiesAllocated[valueSecurity.SecurityId])
						TotalValueAllocated[valueSecurity.SecurityId] = totalValueToAllocate
						fmt.Println(valueSecurity.SecurityId + ": " , TotalValueAllocated[valueSecurity.SecurityId])
						/*TotalValuePledgee += totalValueToAllocate
						fmt.Println("TotalValuePledgee: "+TotalValuePledgee)*/
					}
				} else{
					// no security to take out of this type of security
				}
			} else {
				// Security cutting done
				// Break from the PledgerLongboxSecuritiesIterator as Pledgee's segregated account balance reached to RQV
				break CombinedSecuritiesIterator
			}
		}

		fmt.Println("Final RQVLeft: ", RQVLeft)
		fmt.Println("ReallocatedSecurities after calculation:")
		fmt.Printf("%#v", ReallocatedSecurities)
		fmt.Println()
		fmt.Println("SecuritiesAllocated after calculation:")
		fmt.Printf("%#v", SecuritiesAllocated)
		fmt.Println()
		fmt.Println("RQVEligibleValueLeft after calculation:")
		fmt.Printf("%#v", RQVEligibleValueLeft)
		fmt.Println()
		if RQVLeft <= 0 {
			//-----------------------------------------------------------------------------

			// Flushing securities from both Accounts
			// remove_securitiesFromAccount
			function = "remove_securitiesFromAccount"

			invokeArgs := util.ToChaincodeArgs(function, PledgerLongboxAccount)
			result, err := stub.InvokeChaincode(AccountChainCode, invokeArgs)
			if err != nil {
				errStr := fmt.Sprintf("Failed to flush "+PledgerLongboxAccount+" from 'Account' chaincode. Got error: %s", err.Error())
				fmt.Printf(errStr)
				return nil, errors.New(errStr)
			}
			fmt.Println(result)
			invokeArgs2 := util.ToChaincodeArgs(function, PledgeeSegregatedAccount)
			result2, err := stub.InvokeChaincode(AccountChainCode, invokeArgs2)
			if err != nil {
				errStr := fmt.Sprintf("Failed to flush "+PledgeeSegregatedAccount+" from 'Account' chaincode. Got error: %s", err.Error())
				fmt.Printf(errStr)
				return nil, errors.New(errStr)
			}
			fmt.Println(result2)
			fmt.Print("Securities removed from accounts")
			//-----------------------------------------------------------------------------

			// Committing the state to Blockchain

			// Function from Account Chaincode for
			functionAddSecurity := "add_security" // Security Object

			pledgerLongboxSecuritiesJson := `[`
			// Update the existing Securities for Pledger Longbox A/c
			for i, valueSecurity := range CombinedSecurities {
				securityQuantity, err := strconv.ParseFloat(valueSecurity.SecuritiesQuantity, 64)
				if err != nil {
					errStr := fmt.Sprintf("Failed to convert SecurityQuantity(string) to SecurityQuantity(int). Got error: %s", err.Error())
					fmt.Printf(errStr)
				}
				totalValue, err := strconv.ParseFloat(valueSecurity.TotalValue, 64)
				if err != nil {
					errStr := fmt.Sprintf("Failed to convert totalValue(string) to totalValue(float). Got error: %s", err.Error())
					fmt.Printf(errStr)
				}
				quantityAllocated := SecuritiesAllocated[valueSecurity.SecurityId]
				fmt.Println("quantityAllocated: ",quantityAllocated)
				totalValueAllocated := TotalValueAllocated[valueSecurity.SecurityId]
				fmt.Println("totalValueAllocated: ",totalValueAllocated)
				newQuantity := securityQuantity - quantityAllocated
				fmt.Println("newQuantity: ",newQuantity)
				newTotalValue := totalValue - totalValueAllocated
				fmt.Println("newTotalValue: ",newTotalValue)
				/*effectiveValueChanged, err := strconv.ParseFloat(valueSecurity.EffectiveValueChanged, 64)
				if err != nil {
					errStr := fmt.Sprintf("Failed to convert effectiveValueChanged(string) to effectiveValueChanged(float64). Got error: %s", err.Error())
					fmt.Printf(errStr)
					return nil, errors.New(errStr)
				}
				fmt.Println(effectiveValueChanged)
				_totalValue := effectiveValueChanged * newQuantity*/
				valueSecurity.TotalValue = strconv.FormatFloat(newTotalValue, 'f', 2, 64)

				if newQuantity <= securityQuantity && quantityAllocated >= 0 {

					invokeArgs := util.ToChaincodeArgs(functionAddSecurity, valueSecurity.SecurityId,
						PledgerLongboxAccount,
						valueSecurity.SecuritiesName,
						strconv.FormatFloat(newQuantity, 'f', 2, 64),
						valueSecurity.SecurityType,
						valueSecurity.CollateralForm,
						valueSecurity.TotalValue,
						valueSecurity.ValuePercentage,
						valueSecurity.MTM,
						valueSecurity.EffectivePercentage,
						valueSecurity.EffectiveValueChanged,
						valueSecurity.Currency)
					fmt.Println(valueSecurity)
					result, err := stub.InvokeChaincode(AccountChainCode, invokeArgs)
					if err != nil {
						errStr := fmt.Sprintf("Failed to update Security from 'Account' chaincode. Got error: %s", err.Error())
						fmt.Printf(errStr)
						return nil, errors.New(errStr)
					}
					fmt.Println(result)
					sec, err := json.Marshal(valueSecurity)
					if err != nil {
						fmt.Println("Error while converting CombinedSecurities struct to string")
					}
					pledgerLongboxSecuritiesJson += string(sec)
				}
				if i < len(CombinedSecurities)-1 {
					pledgerLongboxSecuritiesJson += `,`
				}

			}

			pledgerLongboxSecuritiesJson += `]`
			compliance_status := "Regulatory Compliant"
			totalValue_Pri := make(map[string]float64)
			eligibleValue_Pub := make(map[string]float64)
			reallocatedSecuritiesJson := `[`
			// Update the new Securities to Pledgee Segregated A/c
			for i, valueSecurity := range ReallocatedSecurities {
				invokeArgs := util.ToChaincodeArgs(functionAddSecurity, valueSecurity.SecurityId,
					PledgeeSegregatedAccount,
					valueSecurity.SecuritiesName,
					valueSecurity.SecuritiesQuantity,
					valueSecurity.SecurityType,
					valueSecurity.CollateralForm,
					valueSecurity.TotalValue,
					valueSecurity.ValuePercentage,
					valueSecurity.MTM,
					valueSecurity.EffectivePercentage,
					valueSecurity.EffectiveValueChanged,
					valueSecurity.Currency)
				fmt.Println(valueSecurity)
				result, err := stub.InvokeChaincode(AccountChainCode, invokeArgs)
				if err != nil {
					errStr := fmt.Sprintf("Failed to update Security from 'Account' chaincode. Got error: %s", err.Error())
					fmt.Printf(errStr)
					return nil, errors.New(errStr)
				}
				fmt.Println(result)
				sec, err := json.Marshal(valueSecurity)
				if err != nil {
					fmt.Println("Error while converting CombinedSecurities struct to string")
				}
				reallocatedSecuritiesJson += string(sec)
				if i < len(ReallocatedSecurities)-1 {
					reallocatedSecuritiesJson += `,`
				}
				ConcentrationLimit_Pub, errBool1 := strconv.ParseFloat(SecurityJSON[valueSecurity.CollateralForm]["Concentration Limit"], 64)
				if errBool1 != nil {
					fmt.Println(errBool1)
				}
				//ConcentrationLimit_Pri := rulesetFetched.Security[valueSecurity.CollateralForm]["Concentration Limit"]
				temp, errBool2 := strconv.ParseFloat(valueSecurity.MTM, 64)
				if errBool2 != nil {
					fmt.Println(errBool2)
				}

				ValuationPercentage_Pub, errBool3 := strconv.ParseFloat(SecurityJSON[valueSecurity.CollateralForm]["Valuation Percentage"], 64)
				if errBool3 != nil {
					fmt.Println(errBool3)
				}

				//ValuationPercentage_Pri := rulesetFetched.Security[valueSecurity.CollateralForm]["Valuation Percentage"]
				effectiveValueChanged_Pri, errBool4 := strconv.ParseFloat(valueSecurity.EffectiveValueChanged, 64)
				if errBool4 != nil {
					fmt.Println(errBool4)
				}
				totalValuePri, errBool5 := strconv.ParseFloat(valueSecurity.TotalValue, 64)
				if errBool5 != nil {
					fmt.Println(errBool5)
				}
				exchange_rate := ConversionRate.Rates[valueSecurity.Currency]
				if valueSecurity.Currency == RQVCurrency {
					exchange_rate = 1
				}
				newMTM :=  temp/exchange_rate
				fmt.Println("newMTM")
				fmt.Println(newMTM)
				// Effective Value =  (MTM(market Value) * valuePercentage)/100
				effectiveValueChangedPub := (newMTM * ValuationPercentage_Pub)/100
				fmt.Println("effectiveValueChangedPub")
				fmt.Println(effectiveValueChangedPub)
				if effectiveValueChangedPub < effectiveValueChanged_Pri{
					compliance_status = "Regulatory Non-Compliant"
				}
				eligibleValuePub := ConcentrationLimit_Pub * RQV
				totalValue_Pri[valueSecurity.CollateralForm] += totalValuePri
				eligibleValue_Pub[valueSecurity.CollateralForm] += eligibleValuePub

			}
			for key := range totalValue_Pri{
				if totalValue_Pri[key] < eligibleValue_Pub[key] {
					compliance_status = "Regulatory Non-Compliant"	
				}
			}

			reallocatedSecuritiesJson += `]`

			//-----------------------------------------------------------------------------

			// Update Transaction data finally

			ConversionRateAsBytes, _ := json.Marshal(ConversionRate) //marshal an emtpy array of strings to clear the index
			ConversionRateAsString := string(ConversionRateAsBytes[:])
			f := "update_transaction"
			invoke_args := util.ToChaincodeArgs(f,
				TransactionData.TransactionId,
				TransactionData.TransactionDate,
				TransactionData.DealID,
				TransactionData.Pledger,
				TransactionData.Pledgee,
				TransactionData.RQV,
				TransactionData.Currency,
				ConversionRateAsString,
				TransactionData.MarginCAllDate,
				"Allocation Successful",
				"Completed",
				compliance_status)
			fmt.Println(TransactionData)
			res, err := stub.InvokeChaincode(DealChaincode, invoke_args)
			if err != nil {
				errStr := fmt.Sprintf("Failed to invoke chaincode. Got error: %s", err.Error())
				fmt.Printf(errStr)
				return nil, errors.New(errStr)
			}
			fmt.Print("Update transaction returned hash: ")
			fmt.Println(res)
			fmt.Println("Successfully updated allocation status to 'Allocation Successful'")
			
			
			reportInJson += `"Pledger Longbox Securities" : ` + pledgerLongboxSecuritiesJson + `,`
			reportInJson += `"Pledgee Segregated Securities" : ` + reallocatedSecuritiesJson + `,`
			reportInJson += `"Allocation Date" : ` + MarginCallTimpestamp + `,`
			reportInJson += `"Allocation Status" : "Allocation Successful"` + `,`
			reportInJson += `"Compliance Status" : "` + compliance_status + `"` 
			reportInJson += `}`
			fmt.Println(reportInJson)

			//Sending Report
			err = stub.SetEvent("evtsender", []byte(reportInJson))
			if err != nil {
				return nil, err
			}
		} else {
			f := "update_transaction"
			invoke_args := util.ToChaincodeArgs(f, TransactionData.TransactionId, TransactionData.TransactionDate, TransactionData.DealID, TransactionData.Pledger, TransactionData.Pledgee, TransactionData.RQV, TransactionData.Currency, "\" \"", TransactionData.MarginCAllDate, "Pending due to insufficient collateral", "Matched","NA")
			fmt.Println(TransactionData)
			result, err := stub.InvokeChaincode(DealChaincode, invoke_args)
			if err != nil {
				errStr := fmt.Sprintf("Failed to invoke chaincode. Got error: %s", err.Error())
				fmt.Printf(errStr)
				return nil, errors.New(errStr)
			}
			fmt.Print("Update transaction returned : ")
			fmt.Println(result)
			fmt.Println("Successfully updated allocation status to 'Pending' due to insufficient collateral'")
			//Send a event to event handler
			tosend := "{ \"transactionId\" : \"" + TransactionData.TransactionId + "\", \"message\" : \"Transaction Allocation updated succcessfully with status 'Pending' due to insufficient collateral.\", \"code\" : \"200\"}"
			err = stub.SetEvent("evtsender", []byte(tosend))
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	}

	fmt.Println("end start_allocation")
	return nil, nil
}
