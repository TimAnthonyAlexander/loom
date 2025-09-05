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
	export class UIFileEntry {
	    name: string;
	    path: string;
	    is_dir: boolean;
	    size?: number;
	    mod_time: string;
	    ignored?: boolean;
	    hidden?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UIFileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.is_dir = source["is_dir"];
	        this.size = source["size"];
	        this.mod_time = source["mod_time"];
	        this.ignored = source["ignored"];
	        this.hidden = source["hidden"];
	    }
	}
	export class UIListDirResult {
	    path: string;
	    entries: UIFileEntry[];
	    is_dir: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UIListDirResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.entries = this.convertValues(source["entries"], UIFileEntry);
	        this.is_dir = source["is_dir"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UIReadFileResult {
	    path: string;
	    content: string;
	    lines: number;
	    language?: string;
	    serverRev?: string;
	
	    static createFrom(source: any = {}) {
	        return new UIReadFileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.content = source["content"];
	        this.lines = source["lines"];
	        this.language = source["language"];
	        this.serverRev = source["serverRev"];
	    }
	}

}

export namespace config {
	
	export class Tab {
	    path: string;
	    line?: number;
	    column?: number;
	
	    static createFrom(source: any = {}) {
	        return new Tab(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.line = source["line"];
	        this.column = source["column"];
	    }
	}
	export class UILayout {
	    sidebar_width?: number;
	    chat_width?: number;
	    open_tabs?: Tab[];
	    active_tab?: string;
	
	    static createFrom(source: any = {}) {
	        return new UILayout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sidebar_width = source["sidebar_width"];
	        this.chat_width = source["chat_width"];
	        this.open_tabs = this.convertValues(source["open_tabs"], Tab);
	        this.active_tab = source["active_tab"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Settings {
	    openai_api_key: string;
	    anthropic_api_key: string;
	    openrouter_api_key: string;
	    ollama_endpoint?: string;
	    last_workspace?: string;
	    last_model?: string;
	    auto_approve_shell?: boolean;
	    auto_approve_edits?: boolean;
	    theme?: string;
	    personality?: string;
	    recent_workspaces?: string[];
	    selected_models?: string[];
	    ui_layout?: UILayout;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.openai_api_key = source["openai_api_key"];
	        this.anthropic_api_key = source["anthropic_api_key"];
	        this.openrouter_api_key = source["openrouter_api_key"];
	        this.ollama_endpoint = source["ollama_endpoint"];
	        this.last_workspace = source["last_workspace"];
	        this.last_model = source["last_model"];
	        this.auto_approve_shell = source["auto_approve_shell"];
	        this.auto_approve_edits = source["auto_approve_edits"];
	        this.theme = source["theme"];
	        this.personality = source["personality"];
	        this.recent_workspaces = source["recent_workspaces"];
	        this.selected_models = source["selected_models"];
	        this.ui_layout = this.convertValues(source["ui_layout"], UILayout);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
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

export namespace memory {
	
	export class Store {
	
	
	    static createFrom(source: any = {}) {
	        return new Store(source);
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

