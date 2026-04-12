export namespace main {
	
	export class SecretEntry {
	    name: string;
	    data: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new SecretEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.data = source["data"];
	    }
	}

}

