<script>
    import { createEventDispatcher } from "svelte";
    import ApiClient from "@/utils/ApiClient.js";
    import CommonHelper from "@/utils/CommonHelper.js";
    import { removeError } from "@/stores/errors.js";
    import { addSuccessToast } from "@/stores/toasts.js";
    import OverlayPanel from "@/components/base/OverlayPanel.svelte";
    import Field from "@/components/base/Field.svelte";
    import CodeEditor from "@/components/base/CodeEditor.svelte";

    const dispatch = createEventDispatcher();

    let panel;
    let isLoading = false;
    let isValidating = false;
    let validationError = "";
    let original = {};
    let formData = {};

    export function show(functionItem) {
        load(functionItem);
        validationError = "";
        return panel?.show();
    }

    export function hide() {
        return panel?.hide();
    }

    function load(functionItem) {
        isLoading = true;

        original = functionItem || {};

        formData = {
            name: original.name || "",
            schema: original.schema || "public",
            body: original.body || "",
            parameters: original.parameters || [],
            returnType: original.returnType || "void",
            language: original.language || "plpgsql",
            volatility: original.volatility || "VOLATILE",
            security: original.security || "INVOKER",
            description: original.description || "",
            isActive: original.isActive !== undefined ? original.isActive : true,
        };

        isLoading = false;
    }

    function save() {
        if (isLoading) {
            return;
        }

        isLoading = true;

        const data = CommonHelper.filterRedactedProps(formData);

        let request;
        if (original.id) {
            request = ApiClient.send(`/api/pg-functions/${original.id}`, {
                method: "PATCH",
                body: data,
            });
        } else {
            request = ApiClient.send("/api/pg-functions", {
                method: "POST",
                body: data,
            });
        }

        return request
            .then((result) => {
                addSuccessToast(
                    original.id
                        ? `Successfully updated function "${result.name}".`
                        : `Successfully created function "${result.name}".`,
                );
                hide();
                dispatch("save", result);
            })
            .catch((err) => {
                ApiClient.error(err);
            })
            .finally(() => {
                isLoading = false;
            });
    }

    function validateFunction() {
        if (!original.id || isValidating) {
            return;
        }

        isValidating = true;
        validationError = "";

        // Create a temporary function object for validation
        const tempFunction = { ...original, ...formData };

        ApiClient.send(`/api/pg-functions/${original.id}/validate`, {
            method: "POST",
            body: tempFunction,
        })
            .then((result) => {
                if (result.valid) {
                    addSuccessToast("Function SQL is valid!");
                } else {
                    validationError = result.error || "Invalid SQL";
                }
            })
            .catch((err) => {
                validationError = err.data?.message || "Validation failed";
            })
            .finally(() => {
                isValidating = false;
            });
    }

    function addParameter() {
        formData.parameters = [...formData.parameters, { name: "", type: "text", mode: "IN", default: "" }];
    }

    function removeParameter(index) {
        formData.parameters = formData.parameters.filter((_, i) => i !== index);
    }

    // Available PostgreSQL data types
    const pgTypes = [
        "text",
        "varchar",
        "char",
        "integer",
        "bigint",
        "smallint",
        "decimal",
        "numeric",
        "real",
        "double precision",
        "boolean",
        "date",
        "time",
        "timestamp",
        "timestamptz",
        "interval",
        "json",
        "jsonb",
        "uuid",
        "bytea",
        "array",
        "record",
    ];

    const languages = ["plpgsql", "sql", "plpython3u", "plperl"];

    const volatilities = ["VOLATILE", "STABLE", "IMMUTABLE"];

    const securities = ["INVOKER", "DEFINER"];

    function getPlaceholderText() {
        const language = formData.language || "plpgsql";
        const returnType = formData.returnType || "void";

        if (language === "plpgsql") {
            if (returnType === "void") {
                return `-- PL/pgSQL void function example
RAISE NOTICE 'Hello World!';`;
            } else if (returnType === "text" || returnType === "varchar") {
                return `-- PL/pgSQL text function example
'Hello World'
-- OR for complex logic:
-- BEGIN
--     RETURN 'Hello World';
-- END`;
            } else if (returnType === "integer" || returnType === "bigint") {
                return `-- PL/pgSQL integer function example
42
-- OR for complex logic:
-- BEGIN
--     RETURN 42;
-- END`;
            } else {
                return `-- PL/pgSQL function example
BEGIN
    -- Your PL/pgSQL code here
    RETURN 'result';
END`;
            }
        } else if (language === "sql") {
            if (returnType === "void") {
                return `-- SQL void function (rare, usually use plpgsql for void)
-- SQL functions typically return values`;
            } else {
                return `-- SQL function example
SELECT 'Hello World'
-- OR simple expression:
-- 'Hello World'`;
            }
        } else if (language === "plpython3u") {
            return `# Python function example
return "Hello World"`;
        } else if (language === "plperl") {
            return `# Perl function example
return "Hello World";`;
        }

        return "-- Write your function body here";
    }
</script>

<OverlayPanel
    bind:this={panel}
    class="overlay-panel-lg function-panel"
    beforeHide={() => {
        formData = {};
        original = {};
        validationError = "";
        return true;
    }}
>
    <svelte:fragment slot="header">
        <h4>
            {original.id ? "Edit function" : "Create function"}
        </h4>
    </svelte:fragment>

    <div class="grid">
        <div class="col-lg-6">
            <Field class="form-field required" name="name" let:uniqueId>
                <label for={uniqueId}>Name</label>
                <input type="text" id={uniqueId} bind:value={formData.name} required disabled={isLoading} />
            </Field>
        </div>

        <div class="col-lg-6">
            <Field class="form-field required" name="schema" let:uniqueId>
                <label for={uniqueId}>Schema</label>
                <input type="text" id={uniqueId} bind:value={formData.schema} required disabled={isLoading} />
            </Field>
        </div>

        <div class="col-lg-6">
            <Field class="form-field required" name="returnType" let:uniqueId>
                <label for={uniqueId}>Return Type</label>
                <input
                    type="text"
                    id={uniqueId}
                    bind:value={formData.returnType}
                    list="pg-types"
                    required
                    disabled={isLoading}
                />
                <datalist id="pg-types">
                    {#each pgTypes as type}
                        <option value={type} />
                    {/each}
                </datalist>
            </Field>
        </div>

        <div class="col-lg-6">
            <Field class="form-field required" name="language" let:uniqueId>
                <label for={uniqueId}>Language</label>
                <select id={uniqueId} bind:value={formData.language} required disabled={isLoading}>
                    {#each languages as lang}
                        <option value={lang}>{lang}</option>
                    {/each}
                </select>
            </Field>
        </div>

        <div class="col-lg-6">
            <Field class="form-field" name="volatility" let:uniqueId>
                <label for={uniqueId}>Volatility</label>
                <select id={uniqueId} bind:value={formData.volatility} disabled={isLoading}>
                    {#each volatilities as vol}
                        <option value={vol}>{vol}</option>
                    {/each}
                </select>
            </Field>
        </div>

        <div class="col-lg-6">
            <Field class="form-field" name="security" let:uniqueId>
                <label for={uniqueId}>Security</label>
                <select id={uniqueId} bind:value={formData.security} disabled={isLoading}>
                    {#each securities as sec}
                        <option value={sec}>{sec}</option>
                    {/each}
                </select>
            </Field>
        </div>

        <div class="col-12">
            <Field class="form-field" name="description" let:uniqueId>
                <label for={uniqueId}>Description</label>
                <textarea id={uniqueId} bind:value={formData.description} disabled={isLoading} rows="2" />
            </Field>
        </div>

        <div class="col-12">
            <Field class="form-field" name="isActive">
                <input type="checkbox" id="isActive" bind:checked={formData.isActive} disabled={isLoading} />
                <label for="isActive">Function is active</label>
            </Field>
        </div>

        <div class="col-12">
            <Field class="form-field" name="parameters">
                <label class="form-field-label">Parameters</label>
                <div class="parameters-wrapper">
                    {#each formData.parameters as param, index}
                        <div class="parameter-row">
                            <div class="parameter-fields">
                                <input
                                    type="text"
                                    placeholder="Parameter name"
                                    bind:value={param.name}
                                    disabled={isLoading}
                                />
                                <input
                                    type="text"
                                    placeholder="Type"
                                    list="pg-types"
                                    bind:value={param.type}
                                    disabled={isLoading}
                                />
                                <select bind:value={param.mode} disabled={isLoading}>
                                    <option value="IN">IN</option>
                                    <option value="OUT">OUT</option>
                                    <option value="INOUT">INOUT</option>
                                </select>
                                <input
                                    type="text"
                                    placeholder="Default value (optional)"
                                    bind:value={param.default}
                                    disabled={isLoading}
                                />
                            </div>
                            <button
                                type="button"
                                class="btn btn-sm btn-circle btn-secondary"
                                on:click={() => removeParameter(index)}
                                disabled={isLoading}
                            >
                                <i class="ri-close-line" />
                            </button>
                        </div>
                    {/each}

                    <button
                        type="button"
                        class="btn btn-sm btn-secondary"
                        on:click={addParameter}
                        disabled={isLoading}
                    >
                        <i class="ri-add-line" />
                        <span class="txt">Add parameter</span>
                    </button>
                </div>
            </Field>
        </div>

        <div class="col-12">
            <Field class="form-field required" name="body">
                <div class="form-field-label">
                    Function Body
                    {#if original.id}
                        <button
                            type="button"
                            class="btn btn-sm btn-secondary m-l-10"
                            on:click={validateFunction}
                            disabled={isLoading || isValidating}
                        >
                            {#if isValidating}
                                <span class="loader loader-sm" />
                            {:else}
                                <i class="ri-check-line" />
                            {/if}
                            <span class="txt">Validate SQL</span>
                        </button>
                    {/if}
                </div>

                {#if validationError}
                    <div class="alert alert-warning m-b-10">
                        <div class="content">
                            <p><strong>Validation Error:</strong></p>
                            <p>{validationError}</p>
                        </div>
                    </div>
                {/if}

                <CodeEditor
                    bind:value={formData.body}
                    language="sql"
                    disabled={isLoading}
                    placeholder={getPlaceholderText()}
                />
            </Field>
        </div>
    </div>

    <svelte:fragment slot="footer">
        <button type="button" class="btn btn-secondary" disabled={isLoading} on:click={() => hide()}>
            <span class="txt">Cancel</span>
        </button>
        <button
            type="button"
            class="btn btn-expanded"
            disabled={isLoading || !formData.name || !formData.body || !formData.returnType}
            on:click={() => save()}
        >
            <span class="txt">
                {original.id ? "Save changes" : "Create function"}
            </span>
        </button>
    </svelte:fragment>
</OverlayPanel>

<style>
    .parameters-wrapper {
        display: flex;
        flex-direction: column;
        gap: 10px;
    }

    .parameter-row {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 10px;
        border: 1px solid var(--baseAlt2Color);
        border-radius: 4px;
        background: var(--baseAlt1Color);
    }

    .parameter-fields {
        display: flex;
        gap: 10px;
        flex: 1;
    }

    .parameter-fields input,
    .parameter-fields select {
        flex: 1;
        min-width: 0;
    }

    .parameter-fields input:first-child {
        flex: 1.5;
    }

    .parameter-fields select {
        flex: 0.8;
    }
</style>
