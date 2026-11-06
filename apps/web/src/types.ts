export type Language = "python" | "node" | "go"

export interface Submission {
    id : number ;
    language : Language;
    code : string ;
    stdout : string ;
    stderr : string ;
    exit_code : number ;
    timed_out : boolean ;
    created_at : string ;
}