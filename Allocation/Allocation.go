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
	//"github.com/hyperledger/fabric/core/chaincode/shim"
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
type SecurityArrayStruct []Securities

func (slice SecurityArrayStruct) Len() int {
	return len(slice)
}

func (slice SecurityArrayStruct) Less(i, j int) bool {
	// Sorting through the field 'ValuePercentage' for now as it contians the Priority
	return slice[i].ValuePercentage < slice[j].ValuePercentage;
}

func (slice SecurityArrayStruct) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

type Ruleset struct{
	Security interface{} `json:"Security"`
	BaseCurrency string `json:"BaseCurrency"`
	EligibleCurrency []string `json:"EligibleCurrency"`
}

// Use as Object.Rates["EUR"]
// Reference [Tested by Pranav] https://play.golang.org/p/j5Act-jN5C
type CurrencyConversion struct{
	Base string `json:"base"`
	Date string `json:"date"`
	Rates map[string ]float32`json:"rates"`
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
	err = stub.PutState(DealIndexStr, jsonAsBytes)
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
	if function == "nil" {												// Read a Allocation by dealId
		return t.nil(stub, args)
	}
	fmt.Println("query did not find func: " + function)						
	errMsg := "{ \"message\" : \"Received unknown function query\", \"code\" : \"503\"}"
	err := stub.SetEvent("errEvent", []byte(errMsg))
	if err != nil {
		return nil, err
	} 
	return nil, nil
}
// ============================================================================================================================
// Start Allocation - create a new Allocation, store into chaincode state
// ============================================================================================================================
func (t *ManageAllocations) start_allocation(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var err error
	if len(args) != 9 {
		errMsg := "{ \"message\" : \"Incorrect number of arguments. Expecting 9\", \"code\" : \"503\"}"
		err = stub.SetEvent("errEvent", []byte(errMsg))
		if err != nil {
			return nil, err
		} 
		return nil, nil
	}
	fmt.Println("start start_allocation")
	
	// Alloting Pramas
	DealChanincode							:= args[0]
	AccountChainCode 						:= args[1]
	APIIP									:= args[2]
	DealID 									:= args[3]
	TransactionID 							:= args[4]
	PledgerLongboxAccount					:= args[5]
	PledgerSegregatedAccount				:= args[6]
	PledgeeLongboxAccount					:= args[7]
	PledgeeSegregatedAccount				:= args[8]
	MarginCallTimpestamp					:= args[9]


	//-----------------------------------------------------------------------------

	// Fetch Deal details from Blockchain
	dealAsBytes, err := stub.GetState(DealID)
	if err != nil {
		errMsg := "{ \"message\" : \"Failed to get state for " + DealID + "\", \"code\" : \"503\"}"
		err = stub.SetEvent("errEvent", []byte(errMsg))
		if err != nil {
			return nil, err
		} 
		return nil, nil
	}
	DealData := Deals{}
	json.Unmarshal(dealAsBytes, &DealData)
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

	Pledger = DealData.Pledger
	Pledgee = DealData.Pledgee
	fmt.Println("Pledger : " , Pledger)
	fmt.Println("Pledgee : " , Pledgee)

	// Fetch Transaction details from Blockchain
	transactionAsBytes, err := stub.GetState(TransactionID)
	if err != nil {
		errMsg := "{ \"message\" : \"Failed to get state for " + TransactionID + "\", \"code\" : \"503\"}"
		err = stub.SetEvent("errEvent", []byte(errMsg))
		if err != nil {
			return nil, err
		} 
		return nil, nil
	}
	TransactionData := Transactions{}
	json.Unmarshal(transactionAsBytes, &TransactionData)
	if TransactionData.TransactionId == TransactionID{
		fmt.Println("Deal found with TransactionID : " + TransactionID)
	}else{
		errMsg := "{ \"message\" : \""+ TransactionID+ " Not Found.\", \"code\" : \"503\"}"
		err = stub.SetEvent("errEvent", []byte(errMsg))
		if err != nil {
			return nil, err
		} 
		return nil, nil
	}

	RQV = TransactionData.RQV
	fmt.Println("RQV : " , RQV)

	//-----------------------------------------------------------------------------

	// Update allocation status to "Allocation in progress"
	function := "update_transaction_AllocationStatus"
	invokeArgs := util.ToChaincodeArgs(function, TransactionID, "Allocation in progress")
	result, err := stub.InvokeChaincode(DealChanincode, invokeArgs)
	if err != nil {
		errStr := fmt.Sprintf("Failed to update Transaction status from 'Deal' chaincode. Got error: %s", err.Error())
		fmt.Printf(errStr)
		return nil, errors.New(errStr)
	}
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

	// Callers should close resp.Body when done reading from it 
	// Defer the closing of the body
	defer resp.Body.Close()

	// Varaible record to be filled with the data from the JSON
	var rulesetFetched Ruleset

	// Use json.Decode for reading streams of JSON data and store it
	if err := json.NewDecoder(resp.Body).Decode(&rulesetFetched); 
	err != nil {
		log.Println(err)
	}
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

	// Callers should close resp.Body when done reading from it 
	// Defer the closing of the body
	defer resp2.Body.Close()

	// Varaible ConversionRate to be filled with the data from the JSON
	var ConversionRate CurrencyConversion

	// Use json.Decode for reading streams of JSON data and store it
	if err := json.NewDecoder(resp2.Body).Decode(&ConversionRate); 
	err != nil {
		fmt.Println(err)
	}
	fmt.Println("Exchange Rate : ")
	fmt.Println(ConversionRate)

	//-----------------------------------------------------------------------------

	// Caluculate eligible Collateral value from RQV
	RQVEligibleValue := make(map[string]float32)

	//Iterating through all the securities present in the ruleset
	for key,value = range rulesetFetched.Security {
		// key = "CommonStocks" && value = [35, 1, 95]
		// value[0] => ConcentrationLimit
		// value[1] => Priority
		// value[2] => ValuationPercentage

		PriorityPri := strconv.Atoi(value[1])
		PriorityPub := strconv.Atoi(SecurityJSON[key]["Priority"])

		ConcentrationLimitPri := strconv.Atoi(value[1])
		ConcentrationLimitPub := strconv.Atoi(SecurityJSON[key]["ConcentrationLimit"])

		ValuationPercentagePri := strconv.Atoi(value[2])
		ValuationPercentagePub := strconv.Atoi(SecurityJSON[key]["ValuationPercentage"])

		// Check if privateset is subset of publicset
		if PriorityPub > PriorityPri && ConcentrationLimitPub > ConcentrationLimitPri && ValuationPercentagePub > ValuationPercentagePri {
			errMsg := "{ \"message\" : \"Security Ruleset out of allows values for: "+ key +".\", \"code\" : \"503\"}"
			err = stub.SetEvent("errEvent", []byte(errMsg))
			if err != nil {
				return nil, err
			} 
		} else {
			RQVEligibleValue[key] = RQV * ConcentrationLimitPri
		}
	}
	fmt.Println("RQVEligibleValue aftyer calculation:")
	fmt.Println("%#v",RQVEligibleValue)


	//-----------------------------------------------------------------------------

	// Fetch Pledger & Pledgee securities for longbox and segregated accounts
	function = "getSecrurities_byAccount"
	
	invokeArgs = util.ToChaincodeArgs(function, PledgeeLongboxAccount)
	PledgeeLongboxSecuritiesString, err := stub.InvokeChaincode(AccountChainCode, invokeArgs)
	invokeArgs = util.ToChaincodeArgs(function, PledgerLongboxAccount)
	PledgerLongboxSecuritiesString, err := stub.InvokeChaincode(AccountChainCode, invokeArgs)

	invokeArgs = util.ToChaincodeArgs(function, PledgerSegregatedAccount)
	PledgerSegregatedSecuritiesString, err := stub.InvokeChaincode(AccountChainCode, invokeArgs)
	invokeArgs = util.ToChaincodeArgs(function, PledgeeSegregatedAccount)
	PledgeeSegregatedSecuritiesString, err := stub.InvokeChaincode(AccountChainCode, invokeArgs)

	/**	Calculate the effective value and total value of each Security present in the Longbox account of the pledger
		and the Segregated account of the pledgee
	*/
	var TotalValuePledgerLongbox, TotalValuePledgeeSegregated, AvailableEligibleCollateral  float32
	var PledgerLongboxSecurities,PledgeeSegregatedSecurities, CombinedSecurities []Securities

	// Make inteface to recieve string. UnnMarshal them extract them and make an array out of them.
	var PledgerLongboxSecuritiesJSON, PledgeeSegregatedSecuritiesJSON interface{}
	json.Unmarshal(PledgerLongboxSecuritiesString, &PledgerLongboxSecuritiesJSON)
	json.Unmarshal(PledgeeSegregatedSecuritiesString, &PledgeeSegregatedSecuritiesJSON)

	TotalValuePledgerLongboxSecurities := make(map[string]float32)
	TotalValuePledgeeSegregatedSecurities := make(map[string]float32)

	//Operations for Pledger Longbox Securities
	for key,value := range PledgerLongboxSecuritiesJSON {
		// Key = Security ID && value = Security Structure
		tempSecurity := Securities{}
		tempSecurity = value

		// Check if Current Collateral Form type is acceptied in ruleset. If not skip it!
		if len(rulesetFetched.Security[tempSecurity.CollateralForm]) > 0 {

			// Storing the Value percentage in the security ruleset data itself
			tempSecurity.EffectivePercentage = rulesetFetched.Security[tempSecurity.CollateralForm][2]

			// Effective Value = Currency conversion rate(to USD) * MTM(market Value)
			tempSecurity.EffectiveValueinUSD = strconv.Itoa(ConversionRate.Rates[tempSecurity.Currency] * strconv.Atoi(tempSecurity.MTM))

			// Adding it to TotalValue
			tempTotal := strconv.Atoi(tempSecurity.EffectiveValueinUSD) * strconv.Atoi(tempSecurity.SecuritiesQuantity)
			tempSecurity.TotalValue = strconv.Itoa(tempTotal)

			// Calculate Total based on Security Type
			TotalValuePledgerLongboxSecurities[tempSecurity.CollateralForm] += tempTotal
			TotalValuePledgerLongbox =+ tempTotal
			AvailableEligibleCollateral =+ tempTotal

			/*	Warning :
				Saving Priority for the Security in filed `ValuePercentage`
				This is just for using the limited sorting application provided by GOlang
				By no chance is this to be stored on Blockchain. 
			*/
			tempSecurity.ValuationPercentage = rulesetFetched.Security[tempSecurity.CollateralForm][1]

			// Append Securities to an array
			PledgerLongboxSecurities = append(PledgerLongboxSecurities,tempSecurity)
			CombinedSecurities = append(CombinedSecurities,tempSecurity)
		}
	}

	// Operations for Pledgee Segregated Account(s)
	for key,value := range PledgeeSegregatedSecuritiesJSON {
		// Key = Security ID && value = Security Structure
		tempSecurity := Securities{}
		tempSecurity = value

		// Check if Current Collateral Form type is acceptied in ruleset. If not skip it!
		if len(rulesetFetched.Security[tempSecurity.CollateralForm]) > 0 {

			// Storing the Value percentage in the security data itself
			tempSecurity.EffectivePercentage = SecurityJSON[key]["ValuationPercentage"]

			// Effective Value = Currency conversion rate(to USD) * MTM(market Value)
			tempSecurity.EffectiveValueinUSD = strconv.Itoa(ConversionRate.Rates[tempSecurity.Currency] * strconv.Atoi(tempSecurity.MTM))

			// Adding it to TotalValue
			tempTotal := strconv.Atoi(tempSecurity.EffectiveValueinUSD) * strconv.Atoi(tempSecurity.SecuritiesQuantity)
			tempSecurity.TotalValue = strconv.Itoa(tempTotal)

			// Calculate Total based on Security Type
			TotalValuePledgeeSegregatedSecurities[tempSecurity.CollateralForm] += tempTotal
			TotalValuePledgeeSegregated += tempTotal
			AvailableEligibleCollateral =+ tempTotal

			/*	Warning :
				Saving Priority for the Security in filed `ValuePercentage`
				This is just for using the limited sorting application provided by GOlang
				By no chance is this to be stored on Blockchain. 
			*/
			tempSecurity.ValuationPercentage = rulesetFetched.Security[tempSecurity.CollateralForm][1]

			// Append Securities to an array
			PledgeeSegregatedSecurities = append(PledgeeSegregatedSecurities, tempSecurity)
			CombinedSecurities = append(CombinedSecurities,tempSecurity)
		}

	}


	//-----------------------------------------------------------------------------

	if AvailableEligibleCollateral < RQV {
		// Value of eligible collateral available in Pledger Long acc + Pledgee Seg acc < RQV 

			// Update the margin call s Allocation Status as Pending due to insufficient collateral
			/*function := "update_transaction_AllocationStatus"
			invokeArgs := util.ToChaincodeArgs(function, TransactionID, "Pending due to insufficient collateral")
			result, err := stub.InvokeChaincode(DealChanincode, invokeArgs)
			if err != nil {
				errStr := fmt.Sprintf("Failed to update Transaction status from 'Deal' chaincode. Got error: %s", err.Error())
				fmt.Printf(errStr)
				return nil, errors.New(errStr)
			}*/

		fmt.Println("Successfully updated allocation status to 'Pending due to insufficient collateral'")

		order:= `{` +
			`"transactionId": "` + TransactionData.TransactionId + `" , ` + 
			`"transactionDate": "` + TransactionData.TransactionDate + `" , ` + 
			`"dealId": "` + TransactionData.dealId + `" , ` + 
			`"pledger": "` + TransactionData.Pledger + `" , ` + 
			`"pledgee": "` + TransactionData.Pledgee + `" , ` + 
			`"rqv": "` + TransactionData.RQV + `" , ` +
	        `"currency": "` + TransactionData.Currency + `" , ` + 
	        `"currencyConversionRate": "` + "" + `" , ` +  
	        `"marginCAllDate": "` + MarginCallTimpestamp + `" , ` + 
	        `"allocationStatus": "` + "Pending due to insufficient collateral" + `" , ` + 
	        `"transactionStatus": "` + "Pending" + `" ` + 
	        `}`
	    err = stub.PutState(TransactionData.TransactionId, [] byte(order)) //store Deal with id as key
	    if err != nil {
	        return nil, err
	    }
	    //Send a event to event handler
	    tosend:= "{ \"transactionId\" : \"" + _transactionId + "\", \"message\" : \"Transaction updated succcessfully with status \", \"code\" : \"200\"}"
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
		
		sort.Sort(PledgerLongboxSecurities)

		sort.Sort(PledgeeSegregatedSecurities)

		sort.Sort(CombinedSecurities)

		//-----------------------------------------------------------------------------

		// Start Allocatin & Rearrangment
		// ReallocatedSecurities -> Structure where securites to reallocate will be stored
		// CombinedSecurities will only be used to read securities in order. Actual Changes will be 
		//	done in PledgerLongboxSecurities & PledgeeSegregatedSecurities

		// RQVEligibleValue[CollateralType] contains the max eligible vaule for each type
		RQVEligibleValueLeft := RQVEligibleValue
		RQVLeft := RQV

		SecuritiesChanged := make(map[string]float32)
		var ReallocatedSecurities []Securities

		// Iterating through all the securities 
		// Label: CombinedSecuritiesIterator --> to be used for break statements
		CombinedSecuritiesIterator: for _,valueSecurity := range CombinedSecurities {
			if RQVLeft > 0 {
				// More Security need to be taken out
				if strconv.Atoi(RQVEligibleValueLeft[valueSecurity.CollateralForm]) >= strconv.Atoi(valueSecurity.MTM) {
					// At least one more this type of collateralForm to be taken out

					if strconv.Atoi(valueSecurity.EffectiveValueinUSD) <= strconv.Atoi(RQVEligibleValueLeft[valueSecurity.CollateralForm]) {
						// All Security of this type will re allocated if RQV has balance

						if strconv.Atoi(valueSecurity.EffectiveValueinUSD) <= RQVLeft {
							// All Security of this type will re allocated as RQV has balance
							
							RQVLeft -=  strconv.Atoi(valueSecurity.EffectiveValueinUSD)
							RQVEligibleValueLeft[valueSecurity.CollateralForm] -= strconv.Atoi(valueSecurity.EffectiveValueinUSD)
							ReallocatedSecurities.append(ReallocatedSecurities, valueSecurity)
							SecuritiesChanged[valueSecurity.SecurityId] = valueSecurity.SecuritiesQuantity



						} else {
							// RQV has insufficient balance to take all securities

							QuantityToTakeout := math.Ceil(RQVLeft / strconv.Atoi(valueSecurity.MTM))
							EffectiveValueinUSDtoAllocate := QuantityToTakeout * strconv.Atoi(valueSecurity.MTM)

							RQVLeft -= EffectiveValueinUSDtoAllocate
							RQVEligibleValueLeft[valueSecurity.CollateralForm] -= EffectiveValueinUSDtoAllocate
							tempSecurity2 := valueSecurity 
							tempSecurity2.SecuritiesQuantity = strconv.Itoa(QuantityToTakeout)
							tempSecurity2.EffectiveValueinUSD = strconv.Itoa(EffectiveValueinUSDtoAllocate)
							ReallocatedSecurities.append(ReallocatedSecurities, valueSecurity)
							SecuritiesChanged[valueSecurity.SecurityId] = QuantityToTakeout
						}
					} else {
						// We can take out more of this type of CollateralForm but not all
						
						QuantityToTakeout := math.Ceil(RQVEligibleValueLeft[valueSecurity.CollateralForm] / strconv.Atoi(valueSecurity.MTM)) 
						EffectiveValueinUSDtoAllocate := QuantityToTakeout * strconv.Atoi(valueSecurity.MTM)

						if EffectiveValueinUSDtoAllocate >= RQVLeft {
							// Can takeout the Securites 

							RQVLeft -= EffectiveValueinUSDtoAllocate
							RQVEligibleValueLeft[valueSecurity.CollateralForm] -= EffectiveValueinUSDtoAllocate
							tempSecurity2 := valueSecurity 
							tempSecurity2.SecuritiesQuantity = strconv.Itoa(QuantityToTakeout)
							tempSecurity2.EffectiveValueinUSD = strconv.Itoa(EffectiveValueinUSDtoAllocate)
							ReallocatedSecurities.append(ReallocatedSecurities, valueSecurity)
							SecuritiesChanged[valueSecurity.SecurityId] = QuantityToTakeout

						} else {
							// Cannot takeout all possble Securities as RQV balance is low
							
							if QuantityToTakeout > math.Ceil(RQVLeft / strconv.Atoi(valueSecurity.MTM)) {
								QuantityToTakeout = math.Ceil(RQVLeft / strconv.Atoi(valueSecurity.MTM)) 	
							}
							EffectiveValueinUSDtoAllocate = QuantityToTakeout * strconv.Atoi(valueSecurity.MTM)

							RQVLeft -= EffectiveValueinUSDtoAllocate
							RQVEligibleValueLeft[valueSecurity.CollateralForm] -= EffectiveValueinUSDtoAllocate
							tempSecurity2 := valueSecurity 
							tempSecurity2.SecuritiesQuantity = strconv.Itoa(QuantityToTakeout)
							tempSecurity2.EffectiveValueinUSD = strconv.Itoa(EffectiveValueinUSDtoAllocate)
							ReallocatedSecurities.append(ReallocatedSecurities, valueSecurity)
							SecuritiesChanged[valueSecurity.SecurityId] = QuantityToTakeout						
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

		// Committing the state to Blockchain

		// Function from Account Chaincode for 
		functionUpdateSecurity 	:= "update_security" 	// Securities Object
		functionDeleteSecurity	:= "delete_security"	// SecurityId, AccountNumber
		functionAddSecurity 	:= "add_security"		// Security Object

		// Update the existing Securities
		for _,valueSecurity := range CombinedSecurities {
			newQuantity := strconv.Atoi(valueSecurity.SecurityQuantity) - strconv.Atoi(SecuritiesChanged[valueSecurity.SecurityId])
			
			if newQuantity == 0 {
				// Delete the security
				invokeArgs := util.ToChaincodeArgs(functionDeleteSecurity, valueSecurity.SecurityId, valueSecurity.AccountNumber)
				result, err := stub.InvokeChaincode(AccountChainCode, invokeArgs)
				if err != nil {
					errStr := fmt.Sprintf("Failed to Securtiy from from 'Account' chaincode. Got error: %s", err.Error())
					fmt.Printf(errStr)
					return nil, errors.New(errStr)
				}
			} else {
				// Update the security Quantity
				order := 	`{`+
					`"securityId": "` + valueSecurity.SecurityId + `" ,`+
					`"accountNumber": "` + valueSecurity.AccountNumber + `" ,`+													
					`"securityName": "` + valueSecurity.SecuritiesName + `" ,`+
					`"securityQuantity": "` + strconv.Itoa(newQuantity) + `" ,`+
					`"securityType": "` + valueSecurity.SecurityType + `" ,`+
					`"collateralForm": "` + valueSecurity.CollateralForm + `" ,`+
					`"totalvalue": "` + valueSecurity.CollateralForm + `" ,`+
					`"valuePercentage": "` + "" + `" ,`+
					`"mtm": "` + valueSecurity.MTM + `" ,`+
					`"effectivePercentage": "` + valueSecurity.EffectivePercentage + `" `+
					`"EffectiveValueinUSD": "` + valueSecurity.EffectiveValueinUSD + `" `+
					`"currency": "` + valueSecurity.Currency + `" ,`+
					`}`

				invokeArgs := util.ToChaincodeArgs(functionUpdateSecurity, order)
				result, err := stub.InvokeChaincode(AccountChainCode, invokeArgs)
				if err != nil {
					errStr := fmt.Sprintf("Failed to update Security from 'Account' chaincode. Got error: %s", err.Error())
					fmt.Printf(errStr)
					return nil, errors.New(errStr)
				}
			}
		}

		for _, valueSecurity := range ReallocatedSecurities {
			// Updating the new Securities
			order := 	`{`+
				`"securityId": "` + valueSecurity.SecurityId + `" ,`+
				`"accountNumber": "` +  + `" ,`+													//TO ADD!!!!
				`"securityName": "` + valueSecurity.SecuritiesName + `" ,`+
				`"securityQuantity": "` + valueSecurity.SecuritiesQuantity + `" ,`+
				`"securityType": "` + valueSecurity.SecurityType + `" ,`+
				`"collateralForm": "` + valueSecurity.CollateralForm + `" ,`+
				`"totalvalue": "` + valueSecurity.CollateralForm + `" ,`+
				`"valuePercentage": "` + "" + `" ,`+
				`"mtm": "` + valueSecurity.MTM + `" ,`+
				`"effectivePercentage": "` + valueSecurity.EffectivePercentage + `" `+
				`"EffectiveValueinUSD": "` + valueSecurity.EffectiveValueinUSD + `" `+
				`"currency": "` + valueSecurity.Currency + `" ,`+
				`}`

			invokeArgs := util.ToChaincodeArgs(functionAddSecurity, order)
			result, err := stub.InvokeChaincode(AccountChainCode, invokeArgs)
			if err != nil {
				errStr := fmt.Sprintf("Failed to update Security from 'Account' chaincode. Got error: %s", err.Error())
				fmt.Printf(errStr)
				return nil, errors.New(errStr)
			}
		}


		//-----------------------------------------------------------------------------

		// Update Transaction data finally

		fmt.Println("Successfully updated allocation status to 'Allocation Successful'")

		order:= `{` +
			`"transactionId": "` + TransactionData.TransactionId + `" , ` + 
			`"transactionDate": "` + TransactionData.TransactionDate + `" , ` + 
			`"dealId": "` + TransactionData.dealId + `" , ` + 
			`"pledger": "` + TransactionData.Pledger + `" , ` + 
			`"pledgee": "` + TransactionData.Pledgee + `" , ` + 
			`"rqv": "` + TransactionData.RQV + `" , ` +
	        `"currency": "` + TransactionData.Currency + `" , ` + 
	        `"currencyConversionRate": "` + ConversionRate + `" , ` +  
	        `"marginCAllDate": "` + MarginCallTimpestamp + `" , ` + 
	        `"allocationStatus": "` + "Allocation Successful" + `" , ` + 
	        `"transactionStatus": "` + "Complete" + `" ` + 
	        `}`
	    err = stub.PutState(TransactionData.TransactionId, [] byte(order)) //store Deal with id as key
	    if err != nil {
	        return nil, err
	    }

	    // Actual return of process end. 
		ret:= "{ \"message\" : \" + TransactionDataTransactionID + " Completed allocation succcessfully.\", \"code\" : \"200\" }"
		return []byte(ret), nil

	}

	fmt.Println("end start_allocation")
	return nil, nil
}