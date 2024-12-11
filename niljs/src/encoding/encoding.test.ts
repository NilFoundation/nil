import {
    ExternalMessageEnvelope,
    hexToBytes,
} from './index.js'


test('encoding', () => {
    const message = new ExternalMessageEnvelope({
        isDeploy: false,
        to: hexToBytes('0x000100000000000000000000000000000FA00CE7'),
        chainId: 1,
        seqno: 0,
        authData: hexToBytes('0x'),
        data: hexToBytes('0x'),
    })
})