import { nanoid } from "nanoid";

let workingWorker: Worker | null = null;

export const run = (code: string) => {
    const codeBlob = new Blob([code], { type: "text/javascript" });
    const codeUrl = URL.createObjectURL(codeBlob);

    if (workingWorker) {
        workingWorker.terminate();
    }

    const id = nanoid(6);
    
    workingWorker = new Worker(codeUrl, {
        type: "module",
        name: id,
    });
}