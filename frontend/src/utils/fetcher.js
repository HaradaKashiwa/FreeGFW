import { useCallback, useEffect, useState } from "react";
import useSWR from "swr";
import { SWRConfig } from 'swr';
import useSWRMutation from "swr/mutation";


export let config = {
  api: undefined,
}

const useApi = () => {
  const maxRetries = 5
  const fetcher = useCallback(async ({ url, method = 'GET', headers = {}, query = null, retries = maxRetries }, b = { arg: {} }) => {
    const options = { headers, method }
    options.headers['content-language'] = (JSON.parse(window?.localStorage?.getItem('lang')) || window.navigator.language)?.split('-')[0]
    const baseUrl = url.indexOf('http') !== -1 ? '' : config?.api;
    if (method !== 'GET') {
      options.headers['Content-Type'] = 'application/json'
      if (b) options.body = JSON.stringify(b.arg)
    }
    let queryString = '';
    if (query) {
      queryString = `?${serializeRequestParams(query)}`
    }
    try {
      let response, result
      response = await fetch(`${baseUrl}${url}${queryString}`, options);
      try {
        result = await response.json()
      } catch (e) {
        result = await response.text()
      }
      if (response.status === 422) {
        if (result?.errors) {
          const error = new Error()
          error.errors = result.errors
          error.status = response.status
          throw error
        }
        throw new Error('未知错误');
      }

      if (response.status !== 200) {
        if (result?.error) {
          const error = new Error(result?.error)
          error.status = response.status
          throw error
        }
        throw new Error('未知错误');
      }

      return result
    } catch (e) {
      throw e
    }
  }, [])

  return {
    fetcher
  }
}

const delay = (ms) => {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

const _setRequestParams = (name, val, params) => {
  if (val === null) {
    params.push(`${name}=`);
    return;
  }
  if (typeof val !== 'undefined') {
    if (typeof val === 'object') {
      for (const k in val) {
        _setRequestParams(name + '[' + k + ']', val[k], params);
      }
    } else {
      params.push(`${name}=${encodeURIComponent(val)}`);
    }
  }
};

export function serializeRequestParams(data) {
  if (!data) return '';
  if (typeof data === 'object' && !(data instanceof Array)) {
    const params = [];
    for (const k in data) {
      _setRequestParams(k, data[k], params);
    }
    return params.join('&');
  }
  return '';
}

export function useGet({ url, option = {}, query = null, run = true }) {
  const { fetcher } = useApi()
  const { data, error, isLoading, isValidating, mutate } = useSWR(run ? {
    url,
    method: 'GET',
    query
  } : null, fetcher, option)
  return {
    data,
    error,
    loaded: !isLoading,
    loading: isValidating,
    refresh: mutate
  }
}
export function useTrigger({ url, method, option = {}, query = null }) {
  const { fetcher } = useApi()
  const onLoadedHandle = e => {
    if (typeof option?.onLoaded === 'function') {
      option?.onLoaded(e)
    }
  }


  const { data, trigger, error, isMutating, } = useSWRMutation({
    url,
    method,
    query
  }, fetcher, {
    onSuccess: onLoadedHandle,
    onError: () => onLoadedHandle,
    ...option
  })
  return {
    data,
    trigger,
    error: error,
    loading: isMutating
  }
}

export function setConfig(newConfig) {
  config = {
    ...config,
    ...newConfig
  }
}

export {
  SWRConfig
}