export namespace models {
	
	export class Message {
	    id: string;
	    content: string;
	    isUser: boolean;
	    // Go type: time
	    timestamp: any;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.content = source["content"];
	        this.isUser = source["isUser"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.type = source["type"];
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
	export class ChatState {
	    messages: Message[];
	    isStreaming: boolean;
	    streamingContent: string;
	    sessionId: string;
	    workspacePath: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.messages = this.convertValues(source["messages"], Message);
	        this.isStreaming = source["isStreaming"];
	        this.streamingContent = source["streamingContent"];
	        this.sessionId = source["sessionId"];
	        this.workspacePath = source["workspacePath"];
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
	export class FileInfo {
	    path: string;
	    name: string;
	    size: number;
	    isDirectory: boolean;
	    language: string;
	    // Go type: time
	    modifiedTime: any;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.isDirectory = source["isDirectory"];
	        this.language = source["language"];
	        this.modifiedTime = this.convertValues(source["modifiedTime"], null);
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
	
	export class ProjectSummary {
	    summary: string;
	    languages: Record<string, number>;
	    fileCount: number;
	    totalLines: number;
	    // Go type: time
	    generatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new ProjectSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.summary = source["summary"];
	        this.languages = source["languages"];
	        this.fileCount = source["fileCount"];
	        this.totalLines = source["totalLines"];
	        this.generatedAt = this.convertValues(source["generatedAt"], null);
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
	export class TaskInfo {
	    id: string;
	    type: string;
	    description: string;
	    status: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    completedAt?: any;
	    error?: string;
	    preview?: string;
	    result?: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.completedAt = this.convertValues(source["completedAt"], null);
	        this.error = source["error"];
	        this.preview = source["preview"];
	        this.result = source["result"];
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
	export class TaskConfirmation {
	    taskInfo: TaskInfo;
	    preview: string;
	    approved: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TaskConfirmation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskInfo = this.convertValues(source["taskInfo"], TaskInfo);
	        this.preview = source["preview"];
	        this.approved = source["approved"];
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

export namespace task {
	
	export class ActionSuggestion {
	    suggested_action: string;
	    reasoning: string;
	    benefits: string[];
	    example_usage: string;
	    confidence_score: number;
	    efficiency_gain: string;
	
	    static createFrom(source: any = {}) {
	        return new ActionSuggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.suggested_action = source["suggested_action"];
	        this.reasoning = source["reasoning"];
	        this.benefits = source["benefits"];
	        this.example_usage = source["example_usage"];
	        this.confidence_score = source["confidence_score"];
	        this.efficiency_gain = source["efficiency_gain"];
	    }
	}
	export class EditIntent {
	    intent_type: string;
	    target_scope: string;
	    content_type: string;
	    change_nature: string;
	    search_pattern: string;
	    confidence: number;
	
	    static createFrom(source: any = {}) {
	        return new EditIntent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.intent_type = source["intent_type"];
	        this.target_scope = source["target_scope"];
	        this.content_type = source["content_type"];
	        this.change_nature = source["change_nature"];
	        this.search_pattern = source["search_pattern"];
	        this.confidence = source["confidence"];
	    }
	}
	export class ActionAnalysis {
	    current_action: string;
	    is_optimal: boolean;
	    analysis_type: string;
	    edit_intent: EditIntent;
	    suggestions: ActionSuggestion[];
	    optimization_tips: string[];
	    pattern_matches: string[];
	    context_warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new ActionAnalysis(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current_action = source["current_action"];
	        this.is_optimal = source["is_optimal"];
	        this.analysis_type = source["analysis_type"];
	        this.edit_intent = this.convertValues(source["edit_intent"], EditIntent);
	        this.suggestions = this.convertValues(source["suggestions"], ActionSuggestion);
	        this.optimization_tips = source["optimization_tips"];
	        this.pattern_matches = source["pattern_matches"];
	        this.context_warnings = source["context_warnings"];
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
	
	export class ContextLine {
	    line_number: number;
	    content: string;
	    is_target: boolean;
	    is_error: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContextLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line_number = source["line_number"];
	        this.content = source["content"];
	        this.is_target = source["is_target"];
	        this.is_error = source["is_error"];
	    }
	}
	export class ContextualError {
	    type: string;
	    message: string;
	    file_path: string;
	    file_exists: boolean;
	    current_lines: number;
	    current_size: number;
	    requested_action: string;
	    requested_start: number;
	    requested_end: number;
	    actual_content: string[];
	    context_lines: ContextLine[];
	    suggestions: string[];
	    required_actions: string[];
	    prevention_tips: string[];
	
	    static createFrom(source: any = {}) {
	        return new ContextualError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.message = source["message"];
	        this.file_path = source["file_path"];
	        this.file_exists = source["file_exists"];
	        this.current_lines = source["current_lines"];
	        this.current_size = source["current_size"];
	        this.requested_action = source["requested_action"];
	        this.requested_start = source["requested_start"];
	        this.requested_end = source["requested_end"];
	        this.actual_content = source["actual_content"];
	        this.context_lines = this.convertValues(source["context_lines"], ContextLine);
	        this.suggestions = source["suggestions"];
	        this.required_actions = source["required_actions"];
	        this.prevention_tips = source["prevention_tips"];
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
	export class PreviewLine {
	    line_number: number;
	    change_type: string;
	    old_content: string;
	    new_content: string;
	    is_target: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PreviewLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line_number = source["line_number"];
	        this.change_type = source["change_type"];
	        this.old_content = source["old_content"];
	        this.new_content = source["new_content"];
	        this.is_target = source["is_target"];
	    }
	}
	export class DryRunPreview {
	    file_path: string;
	    file_exists: boolean;
	    current_lines: number;
	    current_size: number;
	    expected_lines: number;
	    expected_size: number;
	    line_delta: number;
	    size_delta: number;
	    preview_lines: PreviewLine[];
	    changes_summary: string;
	    safety_warnings: string[];
	    recommended_actions: string[];
	
	    static createFrom(source: any = {}) {
	        return new DryRunPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file_path = source["file_path"];
	        this.file_exists = source["file_exists"];
	        this.current_lines = source["current_lines"];
	        this.current_size = source["current_size"];
	        this.expected_lines = source["expected_lines"];
	        this.expected_size = source["expected_size"];
	        this.line_delta = source["line_delta"];
	        this.size_delta = source["size_delta"];
	        this.preview_lines = this.convertValues(source["preview_lines"], PreviewLine);
	        this.changes_summary = source["changes_summary"];
	        this.safety_warnings = source["safety_warnings"];
	        this.recommended_actions = source["recommended_actions"];
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
	
	export class ValidationSummary {
	    is_valid: boolean;
	    error_count: number;
	    warning_count: number;
	    hint_count: number;
	    critical_errors: string[];
	    validator_used: string;
	    process_time_ms: number;
	    rollback_triggered: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ValidationSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.is_valid = source["is_valid"];
	        this.error_count = source["error_count"];
	        this.warning_count = source["warning_count"];
	        this.hint_count = source["hint_count"];
	        this.critical_errors = source["critical_errors"];
	        this.validator_used = source["validator_used"];
	        this.process_time_ms = source["process_time_ms"];
	        this.rollback_triggered = source["rollback_triggered"];
	    }
	}
	export class LineDiffEntry {
	    line_number: number;
	    change_type: string;
	    old_content: string;
	    new_content: string;
	    context: string;
	
	    static createFrom(source: any = {}) {
	        return new LineDiffEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line_number = source["line_number"];
	        this.change_type = source["change_type"];
	        this.old_content = source["old_content"];
	        this.new_content = source["new_content"];
	        this.context = source["context"];
	    }
	}
	export class EditSummary {
	    file_path: string;
	    edit_type: string;
	    lines_added: number;
	    lines_removed: number;
	    lines_modified: number;
	    total_lines: number;
	    characters_added: number;
	    characters_removed: number;
	    summary: string;
	    was_successful: boolean;
	    is_identical_content: boolean;
	    lines_before: number;
	    lines_after: number;
	    file_size_before: number;
	    file_size_after: number;
	    detailed_diff: LineDiffEntry[];
	    validation_summary?: ValidationSummary;
	
	    static createFrom(source: any = {}) {
	        return new EditSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file_path = source["file_path"];
	        this.edit_type = source["edit_type"];
	        this.lines_added = source["lines_added"];
	        this.lines_removed = source["lines_removed"];
	        this.lines_modified = source["lines_modified"];
	        this.total_lines = source["total_lines"];
	        this.characters_added = source["characters_added"];
	        this.characters_removed = source["characters_removed"];
	        this.summary = source["summary"];
	        this.was_successful = source["was_successful"];
	        this.is_identical_content = source["is_identical_content"];
	        this.lines_before = source["lines_before"];
	        this.lines_after = source["lines_after"];
	        this.file_size_before = source["file_size_before"];
	        this.file_size_after = source["file_size_after"];
	        this.detailed_diff = this.convertValues(source["detailed_diff"], LineDiffEntry);
	        this.validation_summary = this.convertValues(source["validation_summary"], ValidationSummary);
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
	export class InteractivePrompt {
	    prompt: string;
	    response: string;
	    is_regex: boolean;
	    optional: boolean;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new InteractivePrompt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prompt = source["prompt"];
	        this.response = source["response"];
	        this.is_regex = source["is_regex"];
	        this.optional = source["optional"];
	        this.description = source["description"];
	    }
	}
	
	
	export class ValidationStage {
	    name: string;
	    status: string;
	    message: string;
	    details: string;
	    suggestions: string[];
	    duration_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new ValidationStage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.details = source["details"];
	        this.suggestions = source["suggestions"];
	        this.duration_ms = source["duration_ms"];
	    }
	}
	export class ProgressiveValidationResult {
	    overall_status: string;
	    current_stage: string;
	    stages: ValidationStage[];
	    dry_run_preview?: DryRunPreview;
	    action_analysis?: ActionAnalysis;
	    can_proceed: boolean;
	    total_duration_ms: number;
	    failure_stage: string;
	    validation_count: number;
	
	    static createFrom(source: any = {}) {
	        return new ProgressiveValidationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.overall_status = source["overall_status"];
	        this.current_stage = source["current_stage"];
	        this.stages = this.convertValues(source["stages"], ValidationStage);
	        this.dry_run_preview = this.convertValues(source["dry_run_preview"], DryRunPreview);
	        this.action_analysis = this.convertValues(source["action_analysis"], ActionAnalysis);
	        this.can_proceed = source["can_proceed"];
	        this.total_duration_ms = source["total_duration_ms"];
	        this.failure_stage = source["failure_stage"];
	        this.validation_count = source["validation_count"];
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
	export class Task {
	    type: string;
	    path?: string;
	    max_lines?: number;
	    start_line?: number;
	    end_line?: number;
	    show_line_numbers?: boolean;
	    diff?: string;
	    content?: string;
	    intent?: string;
	    loom_edit_command?: boolean;
	    target_line?: number;
	    target_start_line?: number;
	    target_end_line?: number;
	    context_validation?: string;
	    start_context?: string;
	    end_context?: string;
	    insert_mode?: string;
	    command?: string;
	    timeout?: number;
	    interactive?: boolean;
	    input_mode?: string;
	    predefined_input?: string[];
	    expected_prompts?: InteractivePrompt[];
	    allow_user_input?: boolean;
	    recursive?: boolean;
	    query?: string;
	    file_types?: string[];
	    exclude_types?: string[];
	    glob_patterns?: string[];
	    exclude_globs?: string[];
	    ignore_case?: boolean;
	    whole_word?: boolean;
	    fixed_string?: boolean;
	    context_before?: number;
	    context_after?: number;
	    max_results?: number;
	    filenames_only?: boolean;
	    count_matches?: boolean;
	    search_hidden?: boolean;
	    use_pcre2?: boolean;
	    search_names?: boolean;
	    fuzzy_match?: boolean;
	    combine_results?: boolean;
	    max_name_results?: number;
	    memory_operation?: string;
	    memory_id?: string;
	    memory_content?: string;
	    memory_tags?: string[];
	    memory_active?: boolean;
	    memory_description?: string;
	    todo_operation?: string;
	    todo_titles?: string[];
	    todo_item_order?: number;
	    dry_run?: boolean;
	    progressive_validation?: boolean;
	    validation_stages?: boolean;
	    skip_final_confirmation?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.path = source["path"];
	        this.max_lines = source["max_lines"];
	        this.start_line = source["start_line"];
	        this.end_line = source["end_line"];
	        this.show_line_numbers = source["show_line_numbers"];
	        this.diff = source["diff"];
	        this.content = source["content"];
	        this.intent = source["intent"];
	        this.loom_edit_command = source["loom_edit_command"];
	        this.target_line = source["target_line"];
	        this.target_start_line = source["target_start_line"];
	        this.target_end_line = source["target_end_line"];
	        this.context_validation = source["context_validation"];
	        this.start_context = source["start_context"];
	        this.end_context = source["end_context"];
	        this.insert_mode = source["insert_mode"];
	        this.command = source["command"];
	        this.timeout = source["timeout"];
	        this.interactive = source["interactive"];
	        this.input_mode = source["input_mode"];
	        this.predefined_input = source["predefined_input"];
	        this.expected_prompts = this.convertValues(source["expected_prompts"], InteractivePrompt);
	        this.allow_user_input = source["allow_user_input"];
	        this.recursive = source["recursive"];
	        this.query = source["query"];
	        this.file_types = source["file_types"];
	        this.exclude_types = source["exclude_types"];
	        this.glob_patterns = source["glob_patterns"];
	        this.exclude_globs = source["exclude_globs"];
	        this.ignore_case = source["ignore_case"];
	        this.whole_word = source["whole_word"];
	        this.fixed_string = source["fixed_string"];
	        this.context_before = source["context_before"];
	        this.context_after = source["context_after"];
	        this.max_results = source["max_results"];
	        this.filenames_only = source["filenames_only"];
	        this.count_matches = source["count_matches"];
	        this.search_hidden = source["search_hidden"];
	        this.use_pcre2 = source["use_pcre2"];
	        this.search_names = source["search_names"];
	        this.fuzzy_match = source["fuzzy_match"];
	        this.combine_results = source["combine_results"];
	        this.max_name_results = source["max_name_results"];
	        this.memory_operation = source["memory_operation"];
	        this.memory_id = source["memory_id"];
	        this.memory_content = source["memory_content"];
	        this.memory_tags = source["memory_tags"];
	        this.memory_active = source["memory_active"];
	        this.memory_description = source["memory_description"];
	        this.todo_operation = source["todo_operation"];
	        this.todo_titles = source["todo_titles"];
	        this.todo_item_order = source["todo_item_order"];
	        this.dry_run = source["dry_run"];
	        this.progressive_validation = source["progressive_validation"];
	        this.validation_stages = source["validation_stages"];
	        this.skip_final_confirmation = source["skip_final_confirmation"];
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
	export class TaskResponse {
	    task: Task;
	    success: boolean;
	    output?: string;
	    actual_content?: string;
	    edit_summary?: EditSummary;
	    error?: string;
	    contextual_error?: ContextualError;
	    progressive_validation?: ProgressiveValidationResult;
	    approved?: boolean;
	    verification_text?: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.task = this.convertValues(source["task"], Task);
	        this.success = source["success"];
	        this.output = source["output"];
	        this.actual_content = source["actual_content"];
	        this.edit_summary = this.convertValues(source["edit_summary"], EditSummary);
	        this.error = source["error"];
	        this.contextual_error = this.convertValues(source["contextual_error"], ContextualError);
	        this.progressive_validation = this.convertValues(source["progressive_validation"], ProgressiveValidationResult);
	        this.approved = source["approved"];
	        this.verification_text = source["verification_text"];
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

