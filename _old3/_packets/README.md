## MQTT Packets Codec

#### Benchmarks
```bash
goos: darwin
goarch: amd64
pkg: github.com/mochi-co/mqtt/packets
BenchmarkNewPacket-4             	16897716	        73.1 ns/op	     144 B/op	       1 allocs/op
BenchmarkNewParser-4             	 6329433	       163 ns/op	     512 B/op	       1 allocs/op
BenchmarkRead-4                  	10650950	       115 ns/op	      64 B/op	       1 allocs/op

BenchmarkDecodeString-4          	90128386	        13.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeBytes-4           	217574468	         5.41 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeByte-4            	1000000000	         0.343 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeUint16-4          	1000000000	         0.332 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeByteBool-4        	1000000000	         0.340 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeBool-4            	1000000000	         0.337 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeBytes-4           	1000000000	         0.336 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeUint16-4          	1000000000	         0.336 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeString-4          	84589165	        13.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnackEncode-4         	36332194	        39.1 ns/op	      15 B/op	       0 allocs/op
BenchmarkConnackDecode-4         	40500982	        28.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnackValidate-4       	1000000000	         0.337 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnectEncode-4         	 8393736	       145 ns/op	      71 B/op	       0 allocs/op
BenchmarkConnectDecode-4         	16927981	        68.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnectValidate-4       	449974868	         2.70 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisconnectEncode-4      	79734340	        16.9 ns/op	       6 B/op	       0 allocs/op
BenchmarkDisconnectDecode-4      	48091165	        25.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisconnectValidate-4    	1000000000	         0.330 ns/op	       0 B/op	       0 allocs/op
BenchmarkFixedHeaderEncode-4     	83697316	        22.2 ns/op	       6 B/op	       0 allocs/op
BenchmarkFixedHeaderDecode-4     	178036213	         6.40 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeLength-4          	159687147	        10.4 ns/op	       3 B/op	       0 allocs/op
BenchmarkRefreshDeadline-4       	10610847	       107 ns/op	       0 B/op	       0 allocs/op
BenchmarkReset-4                 	24923192	        48.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkReadFixedHeader-4       	22633785	        53.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingreqEncode-4         	78809928	        18.6 ns/op	       6 B/op	       0 allocs/op
BenchmarkPingreqDecode-4         	42101839	        29.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingreqValidate-4       	1000000000	         0.500 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingrespEncode-4        	77277940	        16.4 ns/op	       7 B/op	       0 allocs/op
BenchmarkPingrespDecode-4        	50585750	        25.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingrespValidate-4      	1000000000	         0.395 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubackEncode-4          	40743432	        35.2 ns/op	      13 B/op	       0 allocs/op
BenchmarkPubackDecode-4          	43006642	        31.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubackValidate-4        	1000000000	         0.386 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubcompEncode-4         	44188626	        45.0 ns/op	      12 B/op	       0 allocs/op
BenchmarkPubcompDecode-4         	32195416	        36.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubcompValidate-4       	1000000000	         0.345 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishEncode-4         	18311235	        64.6 ns/op	      31 B/op	       0 allocs/op
BenchmarkPublishDecode-4         	21433636	        53.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishCopy-4           	1000000000	         0.358 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishValidate-4       	901116214	         1.33 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrecEncode-4          	40544341	        29.8 ns/op	      13 B/op	       0 allocs/op
BenchmarkPubrecDecode-4          	40263987	        28.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrecValidate-4        	1000000000	         0.340 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrelEncode-4          	41809491	        28.9 ns/op	      13 B/op	       0 allocs/op
BenchmarkPubrelDecode-4          	42216692	        29.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrelValidate-4        	1000000000	         0.329 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubackEncode-4          	29528050	        40.0 ns/op	      18 B/op	       0 allocs/op
BenchmarkSubackDecode-4          	38786544	        31.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubackValidate-4        	1000000000	         0.338 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubscribeEncode-4       	 7494501	       155 ns/op	      73 B/op	       0 allocs/op
BenchmarkSubscribeDecode-4       	 2645653	       395 ns/op	     267 B/op	       0 allocs/op
BenchmarkSubscribeValidate-4     	1000000000	         0.487 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewFixedHeader-4        	1000000000	         0.343 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnsubackEncode-4        	42987544	        29.2 ns/op	      12 B/op	       0 allocs/op
BenchmarkUnsubackDecode-4        	44304354	        27.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnsubackValidate-4      	1000000000	         0.335 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnsubscribeEncode-4     	 7955698	       148 ns/op	      76 B/op	       0 allocs/op
BenchmarkUnsubscribeDecode-4     	 3199126	       360 ns/op	     258 B/op	       0 allocs/op
BenchmarkUnsubscribeValidate-4   	1000000000	         0.456 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/mochi-co/mqtt/packets	71.055s


```
