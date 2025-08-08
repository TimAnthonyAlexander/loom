export namespace adapter {
	
	export class Config {
	    Provider: string;
	    Model: string;
	    APIKey: string;
	    Endpoint: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Provider = source["Provider"];
	        this.Model = source["Model"];
	        this.APIKey = source["APIKey"];
	        this.Endpoint = source["Endpoint"];
	    }
	}

}

export namespace bridge {
	
	export class App {
	
	
	    static createFrom(source: any = {}) {
	        return new App(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace config {
	
	export class Settings {
	    openai_api_key: string;
	    anthropic_api_key: string;
	    ollama_endpoint?: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.openai_api_key = source["openai_api_key"];
	        this.anthropic_api_key = source["anthropic_api_key"];
	        this.ollama_endpoint = source["ollama_endpoint"];
	    }
	}

}

export namespace engine {
	
	export class Engine {
	
	
	    static createFrom(source: any = {}) {
	        return new Engine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace tool {
	
	export class Registry {
	
	
	    static createFrom(source: any = {}) {
	        return new Registry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

