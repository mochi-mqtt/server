mqtt


## Benchmarks

```
BenchmarkNewClients-4          	177878118	         6.81 ns/op	       0 B/op	       0 allocs/op
BenchmarkClientsAdd-4          	24409328	        47.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkClientsGet-4          	56412216	        21.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkClientsLen-4          	75768160	        16.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkClientsDelete-4       	32501238	        36.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewClient-4           	 3057993	       374 ns/op	     320 B/op	       5 allocs/op
BenchmarkNextPacketID-4        	100000000	        11.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkInFlightSet-4         	 4575694	       252 ns/op	      96 B/op	       2 allocs/op
BenchmarkInFlightGet-4         	25724070	        45.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkNew-4                 	 3227482	       384 ns/op	     440 B/op	       7 allocs/op
BenchmarkServerAddListener-4   	 7300707	       163 ns/op	       0 B/op	       0 allocs/op
BenchmarkServerServe-4         	  444097	      3037 ns/op	     552 B/op	       4 allocs/op
BenchmarkServerClose-4         	 1233651	       833 ns/op	     192 B/op	       3 allocs/op
```

```
BenchmarkNewListeners-4        	100000000	        10.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkAddListener-4         	18383103	        64.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetListener-4         	54720651	        21.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkLenListener-4         	70944740	        16.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkDeleteListener-4      	33414661	        36.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkServeListener-4       	  578126	      3569 ns/op	     529 B/op	       2 allocs/op
BenchmarkServeAllListeners-4   	  266442	     24929 ns/op	    1592 B/op	       7 allocs/op
BenchmarkCloseListener-4       	 5804686	       211 ns/op	      96 B/op	       1 allocs/op
BenchmarkCloseAllListeners-4   	 8264860	       392 ns/op	      96 B/op	       1 allocs/op
BenchmarkNewTCP-4              	31808517	       247 ns/op	      96 B/op	       1 allocs/op
BenchmarkTCPSetConfig-4        	16300988	       134 ns/op	      32 B/op	       1 allocs/op
BenchmarkTCPID-4               	74531133	        16.3 ns/op	       0 B/op	       0 allocs/op
```

```
BenchmarkAllowAuth-4      	1000000000	         0.335 ns/op	       0 B/op	       0 allocs/op
BenchmarkAllowACL-4       	1000000000	         0.351 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisallowAuth-4   	1000000000	         0.340 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisallowACL-4    	1000000000	         0.345 ns/op	       0 B/op	       0 allocs/op
```

```
BenchmarkNew-4               	43395020	        25.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkPoperate-4          	 4947649	       246 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubscribe-4         	 4079262	       301 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnsubscribe-4       	 2971792	       406 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubscribers-4       	 2497588	       462 ns/op	       0 B/op	       0 allocs/op
BenchmarkIsolateParticle-4   	22714515	        48.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkRetainMessage-4     	 4361300	       285 ns/op	       0 B/op	       0 allocs/op
BenchmarkMessages-4          	 3255422	       359 ns/op	       0 B/op	       0 allocs/op
```

``` 
BenchmarkDecodeString-4          	91598564	        14.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeBytes-4           	203964187	         5.69 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeByte-4            	1000000000	         0.344 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeUint16-4          	1000000000	         0.354 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeByteBool-4        	1000000000	         0.334 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeBool-4            	1000000000	         0.342 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeBytes-4           	1000000000	         0.343 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeUint16-4          	1000000000	         0.334 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeString-4          	92204749	        13.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnackEncode-4         	39705741	        37.3 ns/op	      13 B/op	       0 allocs/op
BenchmarkConnackDecode-4         	41083712	        30.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnackValidate-4       	1000000000	         0.340 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnectEncode-4         	 8417053	       143 ns/op	      71 B/op	       0 allocs/op
BenchmarkConnectDecode-4         	17194197	        69.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnectValidate-4       	426824306	         2.72 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisconnectEncode-4      	76213407	        17.9 ns/op	       7 B/op	       0 allocs/op
BenchmarkDisconnectDecode-4      	45947854	        27.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisconnectValidate-4    	1000000000	         0.341 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewFixedHeader-4        	1000000000	         0.338 ns/op	       0 B/op	       0 allocs/op
BenchmarkFixedHeaderEncode-4     	81761335	        16.1 ns/op	       6 B/op	       0 allocs/op
BenchmarkFixedHeaderDecode-4     	218783850	         5.33 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeLength-4          	165308509	         7.27 ns/op	       3 B/op	       0 allocs/op
BenchmarkNewParser-4             	1000000000	         0.335 ns/op	       0 B/op	       0 allocs/op
BenchmarkRefreshDeadline-4       	11080314	       108 ns/op	       0 B/op	       0 allocs/op
BenchmarkReadFixedHeader-4       	19713309	        56.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkRead-4                  	 9818818	       118 ns/op	      64 B/op	       1 allocs/op
BenchmarkPingreqEncode-4         	81325734	        16.1 ns/op	       6 B/op	       0 allocs/op
BenchmarkPingreqDecode-4         	48177753	        27.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingreqValidate-4       	1000000000	         0.330 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingrespEncode-4        	79648717	        16.1 ns/op	       6 B/op	       0 allocs/op
BenchmarkPingrespDecode-4        	42992227	        29.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingrespValidate-4      	1000000000	         0.336 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubackEncode-4          	41951112	        30.3 ns/op	      13 B/op	       0 allocs/op
BenchmarkPubackDecode-4          	37471957	        32.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubackValidate-4        	1000000000	         0.341 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubcompEncode-4         	40825065	        29.8 ns/op	      13 B/op	       0 allocs/op
BenchmarkPubcompDecode-4         	43835835	        28.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubcompValidate-4       	1000000000	         0.350 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishEncode-4         	21782222	        57.2 ns/op	      26 B/op	       0 allocs/op
BenchmarkPublishDecode-4         	23188689	        49.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishCopy-4           	1000000000	         0.331 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishValidate-4       	884320736	         1.33 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrecEncode-4          	41665164	        29.3 ns/op	      13 B/op	       0 allocs/op
BenchmarkPubrecDecode-4          	46009879	        26.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrecValidate-4        	1000000000	         0.329 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrelEncode-4          	42524100	        28.7 ns/op	      12 B/op	       0 allocs/op
BenchmarkPubrelDecode-4          	45454832	        26.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrelValidate-4        	1000000000	         0.335 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubackEncode-4          	29729150	        39.6 ns/op	      18 B/op	       0 allocs/op
BenchmarkSubackDecode-4          	35486103	        34.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubackValidate-4        	1000000000	         0.331 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubscribeEncode-4       	 7566187	       156 ns/op	      73 B/op	       0 allocs/op
BenchmarkSubscribeDecode-4       	 2725272	       376 ns/op	     259 B/op	       0 allocs/op
BenchmarkSubscribeValidate-4     	1000000000	         0.469 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnsubackEncode-4        	43919427	        28.8 ns/op	      12 B/op	       0 allocs/op
BenchmarkUnsubackDecode-4        	42650955	        28.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnsubackValidate-4      	1000000000	         0.340 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnsubscribeEncode-4     	 7185207	       157 ns/op	      84 B/op	       0 allocs/op
BenchmarkUnsubscribeDecode-4     	 3020132	       367 ns/op	     274 B/op	       0 allocs/op
BenchmarkUnsubscribeValidate-4   	1000000000	         0.471 ns/op	       0 B/op	       0 allocs/op
```

```
BenchmarkNewBytesBuffersPool-4   	364001412	         3.05 ns/op	       0 B/op	       0 allocs/op
BenchmarkBytesBuffersPoolGet-4   	 1857973	       659 ns/op	     646 B/op	       3 allocs/op
BenchmarkBytesBuffersPoolPut-4   	 2292727	       548 ns/op	     602 B/op	       2 allocs/op
```