mqtt


## Benchmarks

```
BenchmarkNewClients-4                 	145755818	         7.92 ns/op	       0 B/op	       0 allocs/op
BenchmarkClientsAdd-4                 	24870105	        55.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkClientsGet-4                 	55802703	        23.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkClientsLen-4                 	70591254	        16.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkClientsDelete-4              	33240973	        37.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewClient-4                  	 2598082	       461 ns/op	     448 B/op	       6 allocs/op
BenchmarkNextPacketID-4               	100000000	        11.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkClientNoteSubscription-4     	24480771	        46.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkClientForgetSubscription-4   	12960447	        95.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkInflightSet-4                	17793058	        67.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkInflightGet-4                	25092826	        46.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkNew-4                        	22918302	        44.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkServerAddListener-4          	 7614312	       160 ns/op	       0 B/op	       0 allocs/op
BenchmarkServerServe-4                	  405841	      4993 ns/op	     556 B/op	       4 allocs/op
BenchmarkServerClose-4                	  781466	      1814 ns/op	     192 B/op	       3 allocs/op
BenchmarkServerProcessPacket-4        	345994209	         7.46 ns/op	       0 B/op	       0 allocs/op
BenchmarkServerProcessConnect-4       	230116939	         5.22 ns/op	       0 B/op	       0 allocs/op
BenchmarkServerProcessDisconnect-4    	214293916	         5.11 ns/op	       0 B/op	       0 allocs/op
BenchmarkServerProcessPingreq-4       	  796363	      2678 ns/op	     459 B/op	       7 allocs/op
BenchmarkServerProcessPublish-4       	 8610757	       237 ns/op	      48 B/op	       1 allocs/op
BenchmarkServerProcessPubrec-4        	44983093	        24.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkServerProcessPubrel-4        	48499730	        25.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkServerProcessPubcomp-4       	46276398	        22.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkServerProcessSubscribe-4     	  590350	      3744 ns/op	     728 B/op	       9 allocs/op
BenchmarkServerProcessUnsubscribe-4   	  508149	      2687 ns/op	     475 B/op	       7 allocs/op
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
BenchmarkDecodeString-4          	78840612	        23.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeBytes-4           	100000000	        13.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeByte-4            	1000000000	         0.978 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeUint16-4          	1000000000	         0.756 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeByteBool-4        	1000000000	         0.900 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeBool-4            	1000000000	         0.585 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeBytes-4           	1000000000	         0.441 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeUint16-4          	1000000000	         0.529 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeString-4          	87987272	        21.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnackEncode-4         	36861868	        38.0 ns/op	      14 B/op	       0 allocs/op
BenchmarkConnackDecode-4         	39480138	        34.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnackValidate-4       	1000000000	         0.374 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnectEncode-4         	 7838436	       136 ns/op	      38 B/op	       0 allocs/op
BenchmarkConnectDecode-4         	14495463	        74.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnectValidate-4       	335426919	         3.51 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisconnectEncode-4      	70712485	        18.8 ns/op	       7 B/op	       0 allocs/op
BenchmarkDisconnectDecode-4      	44808872	        29.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisconnectValidate-4    	1000000000	         0.348 ns/op	       0 B/op	       0 allocs/op
BenchmarkFixedHeaderEncode-4     	78281166	        32.3 ns/op	       6 B/op	       0 allocs/op
BenchmarkFixedHeaderDecode-4     	100000000	        12.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeLength-4          	100000000	        15.1 ns/op	       2 B/op	       0 allocs/op
BenchmarkNewParser-4             	1000000000	         0.960 ns/op	       0 B/op	       0 allocs/op
BenchmarkRefreshDeadline-4       	 9184636	       131 ns/op	       0 B/op	       0 allocs/op
BenchmarkReadFixedHeader-4       	18477132	        61.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkRead-4                  	 5692833	       204 ns/op	      96 B/op	       2 allocs/op
BenchmarkPingreqEncode-4         	82176246	        15.7 ns/op	       6 B/op	       0 allocs/op
BenchmarkPingreqDecode-4         	44557917	        31.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingreqValidate-4       	1000000000	         0.338 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingrespEncode-4        	79194856	        16.4 ns/op	       6 B/op	       0 allocs/op
BenchmarkPingrespDecode-4        	48515474	        43.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingrespValidate-4      	1000000000	         0.449 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubackEncode-4          	29720542	        46.3 ns/op	       9 B/op	       0 allocs/op
BenchmarkPubackDecode-4          	23842455	       107 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubackValidate-4        	1000000000	         0.373 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubcompEncode-4         	39364371	        34.0 ns/op	      13 B/op	       0 allocs/op
BenchmarkPubcompDecode-4         	35092684	        31.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubcompValidate-4       	1000000000	         0.335 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishEncode-4         	15284678	        66.6 ns/op	      18 B/op	       0 allocs/op
BenchmarkPublishDecode-4         	23294798	        53.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishCopy-4           	1000000000	         0.376 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishValidate-4       	779486254	         1.44 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrecEncode-4          	40375525	        34.6 ns/op	      13 B/op	       0 allocs/op
BenchmarkPubrecDecode-4          	37656330	        28.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrecValidate-4        	1000000000	         0.378 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrelEncode-4          	36370036	        32.8 ns/op	      15 B/op	       0 allocs/op
BenchmarkPubrelDecode-4          	38191740	        29.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrelValidate-4        	1000000000	         0.394 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubackEncode-4          	19691620	        59.6 ns/op	      27 B/op	       0 allocs/op
BenchmarkSubackDecode-4          	26178052	        40.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubackValidate-4        	1000000000	         0.366 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubscribeEncode-4       	 6134076	       187 ns/op	      90 B/op	       0 allocs/op
BenchmarkSubscribeDecode-4       	 2296219	       472 ns/op	     303 B/op	       0 allocs/op
BenchmarkSubscribeValidate-4     	1000000000	         0.585 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnsubackEncode-4        	39235142	        32.8 ns/op	      13 B/op	       0 allocs/op
BenchmarkUnsubackDecode-4        	34655808	        42.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnsubackValidate-4      	1000000000	         0.458 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnsubscribeEncode-4     	 7313114	       171 ns/op	      83 B/op	       0 allocs/op
BenchmarkUnsubscribeDecode-4     	 3187930	       372 ns/op	     259 B/op	       0 allocs/op
BenchmarkUnsubscribeValidate-4   	1000000000	         0.544 ns/op	       0 B/op	       0 allocs/op
```

