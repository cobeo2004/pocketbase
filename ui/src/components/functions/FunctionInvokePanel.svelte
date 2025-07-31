<script>
    import { createEventDispatcher } from "svelte";
    import ApiClient from "@/utils/ApiClient.js";
    import { addSuccessToast } from "@/stores/toasts.js";
    import OverlayPanel from "@/components/base/OverlayPanel.svelte";
    import Field from "@/components/base/Field.svelte";
    import CodeEditor from "@/components/base/CodeEditor.svelte";

    const dispatch = createEventDispatcher();

    let panel;
    let isLoading = false;
    let functionItem = {};
    let parameters = {};
    let result = null;
    let error = null;

    export function show(func) {
        functionItem = func;
        parameters = {};
        result = null;
        error = null;

        // Initialize parameters with default values
        if (func.parameters) {
            func.parameters.forEach((param) => {
                if (param.mode !== "OUT") {
                    parameters[param.name] = param.default || "";
                }
            });
        }

        return panel?.show();
    }

    export function hide() {
        return panel?.hide();
    }

    function invokeFunction() {
        if (isLoading) {
            return;
        }

        isLoading = true;
        error = null;
        result = null;

        ApiClient.send(`/api/pg-functions/${functionItem.id}/invoke`, {
            method: "POST",
            body: parameters,
        })
            .then((response) => {
                result = response.result;
                addSuccessToast("Function executed successfully!");
            })
            .catch((err) => {
                error = err.data?.message || err.message || "Function execution failed";
                ApiClient.error(err);
            })
            .finally(() => {
                isLoading = false;
            });
    }

    function formatResult(value) {
        if (value === null || value === undefined) {
            return "null";
        }
        if (typeof value === "object") {
            return JSON.stringify(value, null, 2);
        }
        return String(value);
    }

    $: inputParameters = functionItem.parameters?.filter((p) => p.mode !== "OUT") || [];
    $: hasInputParameters = inputParameters.length > 0;
</script>

<OverlayPanel
    bind:this={panel}
    class="overlay-panel-lg function-invoke-panel"
    beforeHide={() => {
        functionItem = {};
        parameters = {};
        result = null;
        error = null;
        return true;
    }}
>
    <svelte:fragment slot="header">
        <h4>
            Test function: {functionItem.name}
        </h4>
        {#if functionItem.signature}
            <p class="txt-sm txt-hint m-t-5">
                {functionItem.signature}
            </p>
        {/if}
    </svelte:fragment>

    <div class="grid">
        {#if functionItem.description}
            <div class="col-12">
                <div class="alert alert-info">
                    <div class="content">
                        <p><strong>Description:</strong></p>
                        <p>{functionItem.description}</p>
                    </div>
                </div>
            </div>
        {/if}

        {#if hasInputParameters}
            <div class="col-12">
                <h6>Parameters</h6>
                <div class="grid">
                    {#each inputParameters as param}
                        <div class="col-lg-6">
                            <Field class="form-field" name={param.name} let:uniqueId>
                                <label for={uniqueId}>
                                    {param.name}
                                    <span class="txt-hint">({param.type})</span>
                                    {#if param.mode !== "IN"}
                                        <span class="txt-sm txt-warning">({param.mode})</span>
                                    {/if}
                                </label>

                                {#if param.type === "boolean"}
                                    <select
                                        id={uniqueId}
                                        bind:value={parameters[param.name]}
                                        disabled={isLoading}
                                    >
                                        <option value="">-- Select --</option>
                                        <option value={true}>true</option>
                                        <option value={false}>false</option>
                                    </select>
                                {:else if param.type.includes("json")}
                                    <textarea
                                        id={uniqueId}
                                        bind:value={parameters[param.name]}
                                        placeholder="Enter JSON value"
                                        disabled={isLoading}
                                        rows="3"
                                    />
                                {:else}
                                    <input
                                        type="text"
                                        id={uniqueId}
                                        bind:value={parameters[param.name]}
                                        placeholder={param.default
                                            ? `Default: ${param.default}`
                                            : `Enter ${param.type} value`}
                                        disabled={isLoading}
                                    />
                                {/if}
                            </Field>
                        </div>
                    {/each}
                </div>
            </div>
        {:else}
            <div class="col-12">
                <div class="alert alert-info">
                    <div class="content">
                        <p>This function has no input parameters.</p>
                    </div>
                </div>
            </div>
        {/if}

        <div class="col-12">
            <div class="flex">
                <button
                    type="button"
                    class="btn btn-expanded"
                    disabled={isLoading || !functionItem.isActive}
                    on:click={invokeFunction}
                >
                    {#if isLoading}
                        <span class="loader loader-sm" />
                    {:else}
                        <i class="ri-play-line" />
                    {/if}
                    <span class="txt">Execute Function</span>
                </button>
            </div>

            {#if !functionItem.isActive}
                <div class="alert alert-warning m-t-10">
                    <div class="content">
                        <p>This function is currently inactive and cannot be executed.</p>
                    </div>
                </div>
            {/if}
        </div>

        {#if result !== null || error}
            <div class="col-12">
                <h6>Result</h6>

                {#if error}
                    <div class="alert alert-danger">
                        <div class="content">
                            <p><strong>Error:</strong></p>
                            <pre>{error}</pre>
                        </div>
                    </div>
                {:else}
                    <div class="alert alert-success">
                        <div class="content">
                            <p><strong>Result:</strong></p>
                            <CodeEditor
                                value={formatResult(result)}
                                language="json"
                                readonly={true}
                                class="result-editor"
                            />
                        </div>
                    </div>
                {/if}
            </div>
        {/if}
    </div>

    <svelte:fragment slot="footer">
        <button type="button" class="btn btn-secondary" disabled={isLoading} on:click={() => hide()}>
            <span class="txt">Close</span>
        </button>
    </svelte:fragment>
</OverlayPanel>

<style>
    :global(.result-editor) {
        max-height: 200px;
        margin-top: 10px;
    }

    pre {
        white-space: pre-wrap;
        word-break: break-word;
        margin: 0;
        font-family: var(--monospaceFontFamily);
        font-size: 0.85em;
    }
</style>
