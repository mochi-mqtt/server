## MQTT Packets Codec

#### Benchmarks
```bash
goos: darwin
goarch: amd64
pkg: github.com/mochi-co/mqtt/packets
BenchmarkDecodeString-4        	93908146	        13.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeBytes-4         	228220797	         5.25 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeByte-4          	1000000000	         0.335 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeUint16-4        	1000000000	         0.329 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeByteBool-4      	1000000000	         0.331 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeBool-4          	1000000000	         0.332 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeBytes-4         	1000000000	         0.327 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeUint16-4        	1000000000	         0.341 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeString-4        	94070353	        12.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnackEncode-4       	40623596	        35.6 ns/op	      13 B/op	       0 allocs/op
BenchmarkConnackDecode-4       	37468688	        32.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnectEncode-4       	 8593335	       143 ns/op	      69 B/op	       0 allocs/op
BenchmarkConnectDecode-4       	17702611	        66.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnectValidate-4     	32963706	        34.0 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisconnectEncode-4    	79222494	        17.5 ns/op	       6 B/op	       0 allocs/op
BenchmarkDisconnectDecode-4    	49523570	        24.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkFixedHeaderEncode-4   	86247478	        16.0 ns/op	       6 B/op	       0 allocs/op
BenchmarkFixedHeaderDecode-4   	235219791	         6.52 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeLength-4        	139435804	        12.1 ns/op	       3 B/op	       0 allocs/op
BenchmarkNewParser-4           	 6747854	       165 ns/op	     512 B/op	       1 allocs/op
BenchmarkRefreshDeadline-4     	11034858	       189 ns/op	       0 B/op	       0 allocs/op
BenchmarkReset-4               	11974035	       153 ns/op	       0 B/op	       0 allocs/op
BenchmarkReadFixedHeader-4     	16009377	        86.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkRead-4                	 8149027	       198 ns/op	      64 B/op	       1 allocs/op
BenchmarkPingreqEncode-4       	77362615	        21.0 ns/op	       7 B/op	       0 allocs/op
BenchmarkPingreqDecode-4       	44813784	        32.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingrespEncode-4      	47898571	        25.8 ns/op	       5 B/op	       0 allocs/op
BenchmarkPingrespDecode-4      	25430264	        68.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubackEncode-4        	22943926	        48.2 ns/op	      11 B/op	       0 allocs/op
BenchmarkPubackDecode-4        	37825020	        35.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubcompEncode-4       	35609434	        37.1 ns/op	      15 B/op	       0 allocs/op
BenchmarkPubcompDecode-4       	38840658	        35.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishEncode-4       	15692988	       114 ns/op	      18 B/op	       0 allocs/op
BenchmarkPublishDecode-4       	19327656	        84.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishCopy-4         	1000000000	         0.594 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishValidate-4     	991284961	         1.15 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrecEncode-4        	38247674	        29.9 ns/op	      14 B/op	       0 allocs/op
BenchmarkPubrecDecode-4        	38628714	        29.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrelEncode-4        	39538930	        35.1 ns/op	      13 B/op	       0 allocs/op
BenchmarkPubrelDecode-4        	35581755	        28.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubackEncode-4        	29551377	        37.9 ns/op	      18 B/op	       0 allocs/op
BenchmarkSubackDecode-4        	33948814	        40.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubscribeEncode-4     	 6846366	       180 ns/op	      80 B/op	       0 allocs/op
BenchmarkSubscribeDecode-4     	 2955309	       382 ns/op	     295 B/op	       0 allocs/op
BenchmarkNewFixedHeader-4      	1000000000	         0.327 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewPacket-4           	18423596	        65.2 ns/op	     144 B/op	       1 allocs/op
BenchmarkUnsubackEncode-4      	37811830	        34.7 ns/op	      14 B/op	       0 allocs/op
BenchmarkUnsubackDecode-4      	37173764	        31.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnsubscribeEncode-4   	 5725280	       204 ns/op	     106 B/op	       0 allocs/op
BenchmarkUnsubscribeDecode-4   	 3386936	       367 ns/op	     244 B/op	       0 allocs/op
PASS
ok  	github.com/mochi-co/mqtt/packets	70.543s
```
