## MQTT Packets Codec

#### Benchmarks
```bash
goos: darwin
goarch: amd64
pkg: github.com/mochi-co/mqtt/packets
BenchmarkDecodeString-4          	92279000	        13.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeBytes-4           	180003661	         5.82 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeByte-4            	1000000000	         0.351 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeUint16-4          	1000000000	         0.331 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeByteBool-4        	1000000000	         0.329 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeBool-4            	1000000000	         0.328 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeBytes-4           	1000000000	         0.331 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeUint16-4          	1000000000	         0.351 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeString-4          	86298631	        13.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnackEncode-4         	38169920	        44.3 ns/op	      14 B/op	       0 allocs/op
BenchmarkConnackDecode-4         	31907284	        34.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnackValidate-4       	1000000000	         0.345 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnectEncode-4         	 8935698	       138 ns/op	      66 B/op	       0 allocs/op
BenchmarkConnectDecode-4         	17962437	        67.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkConnectValidate-4       	32865400	        46.0 ns/op	      16 B/op	       1 allocs/op
BenchmarkDisconnectEncode-4      	74182669	        21.6 ns/op	       7 B/op	       0 allocs/op
BenchmarkDisconnectDecode-4      	43139491	        37.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkDisconnectValidate-4    	1000000000	         0.536 ns/op	       0 B/op	       0 allocs/op
BenchmarkFixedHeaderEncode-4     	74693355	        19.4 ns/op	       7 B/op	       0 allocs/op
BenchmarkFixedHeaderDecode-4     	226088620	         5.46 ns/op	       0 B/op	       0 allocs/op
BenchmarkEncodeLength-4          	180389052	         7.04 ns/op	       3 B/op	       0 allocs/op
BenchmarkNewParser-4             	 7387656	       139 ns/op	     512 B/op	       1 allocs/op
BenchmarkRefreshDeadline-4       	10294341	       111 ns/op	       0 B/op	       0 allocs/op
BenchmarkReset-4                 	24490851	        48.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkReadFixedHeader-4       	21249194	        55.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkRead-4                  	 9995664	       118 ns/op	      64 B/op	       1 allocs/op
BenchmarkPingreqEncode-4         	76834284	        16.1 ns/op	       7 B/op	       0 allocs/op
BenchmarkPingreqDecode-4         	46166296	        26.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingreqValidate-4       	1000000000	         0.336 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingrespEncode-4        	78247909	        15.9 ns/op	       6 B/op	       0 allocs/op
BenchmarkPingrespDecode-4        	43962834	        27.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkPingrespValidate-4      	1000000000	         0.332 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubackEncode-4          	41049776	        29.9 ns/op	      13 B/op	       0 allocs/op
BenchmarkPubackDecode-4          	43284777	        28.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubackValidate-4        	1000000000	         0.332 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubcompEncode-4         	41348029	        29.7 ns/op	      13 B/op	       0 allocs/op
BenchmarkPubcompDecode-4         	41050764	        29.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubcompValidate-4       	1000000000	         0.338 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishEncode-4         	20109715	        59.6 ns/op	      28 B/op	       0 allocs/op
BenchmarkPublishDecode-4         	23571778	        49.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishCopy-4           	1000000000	         0.332 ns/op	       0 B/op	       0 allocs/op
BenchmarkPublishValidate-4       	1000000000	         0.989 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrecEncode-4          	41449594	        29.7 ns/op	      13 B/op	       0 allocs/op
BenchmarkPubrecDecode-4          	42904152	        28.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrecValidate-4        	1000000000	         0.333 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrelEncode-4          	42544890	        29.0 ns/op	      12 B/op	       0 allocs/op
BenchmarkPubrelDecode-4          	41451946	        29.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkPubrelValidate-4        	1000000000	         0.330 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubackEncode-4          	26801430	        39.2 ns/op	      20 B/op	       0 allocs/op
BenchmarkSubackDecode-4          	34504112	        34.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubackValidate-4        	1000000000	         0.329 ns/op	       0 B/op	       0 allocs/op
BenchmarkSubscribeEncode-4       	 7277469	       163 ns/op	      75 B/op	       0 allocs/op
BenchmarkSubscribeDecode-4       	 2601225	       386 ns/op	     271 B/op	       0 allocs/op
BenchmarkSubscribeValidate-4     	1000000000	         0.462 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewFixedHeader-4        	1000000000	         0.330 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewPacket-4             	15078193	        69.7 ns/op	     144 B/op	       1 allocs/op
BenchmarkUnsubackEncode-4        	44562201	        28.8 ns/op	      12 B/op	       0 allocs/op
BenchmarkUnsubackDecode-4        	39644432	        31.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnsubackValidate-4      	1000000000	         0.335 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnsubscribeEncode-4     	 7765140	       149 ns/op	      78 B/op	       0 allocs/op
BenchmarkUnsubscribeDecode-4     	 3255702	       316 ns/op	     254 B/op	       0 allocs/op
BenchmarkUnsubscribeValidate-4   	1000000000	         0.463 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/mochi-co/mqtt/packets	68.357s

```
