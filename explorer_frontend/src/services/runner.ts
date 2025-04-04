export const run = (code: string) => {
    const codeBlob = new Blob([code], { type: "text/javascript" });
    
    new Worker(URL.createObjectURL(codeBlob));
}