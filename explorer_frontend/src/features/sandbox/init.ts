import { sandboxRoute } from "../routing";
import { $address, $amount, changeAddress, changeAmount } from "./models/model";

$amount.on(changeAmount, (_, amount) => amount);
$address.on(changeAddress, (_, address) => address);
$amount.reset(sandboxRoute.open);
$address.reset(sandboxRoute.open);
