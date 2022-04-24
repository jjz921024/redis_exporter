package exporter

import (
	"io/ioutil"
	"testing"
)

var pwd = "7eb13069d12093865ed5bb045de366e5a97143ed63ad13abac90dd18bc9ff62451a28a9778d86ac3aacc92df2a0eebb8b11d8d2ec85b987feeee49fa46969b2180d1774280e944ba350937a80150d06f7287a09047f85651eeb16937e4c9cff2cb5cd5ecfb48a36448b3afd27eb0ee0759ee47e2f23ed330580e35ecc9e80995d75e1fbd86a89049f64776d21c104b5d41195279e341116ceb9d6a49d7a8e9c6f440130e33cd23b3b429931fa50a7438e4475b0e44d03a581359883ffc2989e1225af03449a1be4df370ad41eeb8e1570022412ddf97fa29ad8f06f3604f68eeb5dbcf0f898ba702a7179045e65694e4a528a2f64ee18a5825345de66ebf7057c0c65e0815458c361c2957139ee0367dc4b5a4882afe4e28c3bf5faf7042b0f82e0bd7811a99535fa23b5d50328c9b9f55c12ebce751752f2ae8be9acec282eb8b40f66728470fea35f32bc1209aac4b8997988f9d4dfe9999141d18de10cc31046db157936bd7c06a81b9b361b07af22bab9cd035fadf6e26844812596bffe363f07ffec1229a7032ecc4c1197592b4a1df280235066e7f3f5db8ef2a28889321dd3fb85826502115b17915090ea875865dba1528f3be01b90157ed134e30cf87a2513f15cf2c2d1f10eb35b37a037dd10a703297b1ec4344da4d46a2bcfa9d69837fa0e851e22aac1c754d0dc454a3551acd93c8775468a165d7de19e61bcc"

func TestRSA(t *testing.T) {
	aompPubKey, err := ioutil.ReadFile("D:\\GolandProjects\\redis_exporter\\conf\\aomp-public.pem")
	if err != nil {
		t.Errorf("read aomp: %s\n", err)
	}
	appPrivKey, err := ioutil.ReadFile("D:\\GolandProjects\\redis_exporter\\conf\\proxy-private.pem")
	if err != nil {
		t.Errorf("read private: %s\n", err)
	}

	if pwd, err = AompPasswordDecrypt(pwd, string(aompPubKey), string(appPrivKey)); err != nil {
		t.Errorf("decrpyt err: %s\n", err)
	}

	t.Logf("pwd: %s\n", pwd)
}
