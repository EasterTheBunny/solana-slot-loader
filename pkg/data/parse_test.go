package data_test

import (
	"testing"

	"github.com/easterthebunny/solana-slot-loader/pkg/data"
)

func TestParseLogs(t *testing.T) {
	logs := []string{
		"Program ComputeBudget111111111111111111111111111111 invoke [1]",
		"Program ComputeBudget111111111111111111111111111111 success",
		"Program ComputeBudget111111111111111111111111111111 invoke [1]",
		"Program ComputeBudget111111111111111111111111111111 success",
		"Program SW1TCH7qEPTdLsDHRgPuMQjbQxKdH2aBStViMFnt64f invoke [1]",
		"Program log: Instruction: AggregatorSaveResult",
		"Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA invoke [2]",
		"Program log: Instruction: Transfer",
		"Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA consumed 4736 of 258278 compute units",
		"Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA success",
		"Program data: Dk7x7N2nVamSe+1k+gNfIU+pCq3Je/Tun7I1vOgqssHkgYj05qfI7I5EtBAAAAAAp2XWZQAAAAAKAAAAAAAAAA==",
		"Program data: A5o8/ZicmX6Se+1k+gNfIU+pCq3Je/Tun7I1vOgqssHkgYj05qfI7AkhAAAAAAAAAAAAAAAAAAAFAAAAjkS0EAAAAACnZdZlAAAAAFZD3ksB9oWPSzwHM3ce5o/+um8Z7t8D4306zc376LvHAAAAAA==",
		"Program data: TOkj7nevZ9aSe+1k+gNfIU+pCq3Je/Tun7I1vOgqssHkgYj05qfI7AAAAAAAAAAAVkPeSwH2hY9LPAczdx7mj/66bxnu3wPjfTrNzfvou8elgqsRAAAAAKWCqxEAAAAA",
		"Program data: cB8z6WFkK/WSe+1k+gNfIU+pCq3Je/Tun7I1vOgqssHkgYj05qfI7AkhAAAAAAAAAAAAAAAAAAAFAAAAjkS0EAAAAACnZdZlAAAAAAAAAAAAAAAA",
		"Program data: TOkj7nevZ9aSe+1k+gNfIU+pCq3Je/Tun7I1vOgqssHkgYj05qfI7NQwAAAAAAAAVkPeSwH2hY9LPAczdx7mj/66bxnu3wPjfTrNzfvou8elgqsRAAAAABmkXNsFAAAA",
		"Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA invoke [2]",
		"Program log: Instruction: Transfer",
		"Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA consumed 4736 of 226588 compute units",
		"Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA success",
		"Program SW1TCH7qEPTdLsDHRgPuMQjbQxKdH2aBStViMFnt64f consumed 81421 of 299700 compute units",
		"Program SW1TCH7qEPTdLsDHRgPuMQjbQxKdH2aBStViMFnt64f success",
	}

	out := data.ParseLogs(logs)
	for _, pLogs := range out {
		for _, log := range pLogs.Logs {
			t.Logf("%s: %s", pLogs.Program, log.Text)
		}
	}
}
