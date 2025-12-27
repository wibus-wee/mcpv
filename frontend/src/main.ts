import {Events} from "@wailsio/runtime";
import { GetVersion } from "../bindings/mcpd/internal/ui/wailsservice";


const resultElement = document.getElementById('result');
const timeElement = document.getElementById('time');

window.doGreet = async () => {
    let name = document.getElementById('name').value;
    if (!name) {
        name = 'anonymous';
    }
    try {
        resultElement.innerText = await GetVersion();
    } catch (err) {
        console.error(err);
    }
}

Events.On('time', (time) => {
    timeElement.innerText = time.data;
});
